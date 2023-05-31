package main

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightninglabs/loop"
	"github.com/lightninglabs/loop/loopdb"
	"github.com/lightninglabs/loop/swap"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/spf13/cobra"
)

type recoverLoopInCommand struct {
	TxID          string
	Vout          uint32
	SwapHash      string
	SweepAddr     string
	FeeRate       uint32
	StartKeyIndex int
	NumTries      int

	APIURL  string
	Publish bool

	LoopDbDir string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newRecoverLoopInCommand() *cobra.Command {
	cc := &recoverLoopInCommand{}
	cc.cmd = &cobra.Command{
		Use: "recoverloopin",
		Short: "Recover a loop in swap that the loop daemon " +
			"is not able to sweep",
		Example: `chantools recoverloopin \
	--txid abcdef01234... \
	--vout 0 \
	--swap_hash abcdef01234... \
	--loop_db_dir /path/to/loop/db/dir \
	--sweep_addr bc1pxxxxxxx \
	--feerate 10`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.TxID, "txid", "", "transaction id of the on-chain "+
			"transaction that created the HTLC",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.Vout, "vout", 0, "output index of the on-chain "+
			"transaction that created the HTLC",
	)
	cc.cmd.Flags().StringVar(
		&cc.SwapHash, "swap_hash", "", "swap hash of the loop in "+
			"swap",
	)
	cc.cmd.Flags().StringVar(
		&cc.LoopDbDir, "loop_db_dir", "", "path to the loop "+
			"database directory, where the loop.db file is located",
	)
	cc.cmd.Flags().StringVar(
		&cc.SweepAddr, "sweep_addr", "", "address to recover "+
			"the funds to",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.FeeRate, "feerate", 0, "fee rate to "+
			"use for the sweep transaction in sat/vByte",
	)
	cc.cmd.Flags().IntVar(
		&cc.NumTries, "num_tries", 1000, "number of tries to "+
			"try to find the correct key index",
	)
	cc.cmd.Flags().IntVar(
		&cc.StartKeyIndex, "start_key_index", 0, "start key index "+
			"to try to find the correct key index",
	)
	cc.cmd.Flags().StringVar(
		&cc.APIURL, "apiurl", defaultAPIURL, "API URL to use (must "+
			"be esplora compatible)",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Publish, "publish", false, "publish sweep TX to the chain "+
			"API instead of just printing the TX",
	)

	cc.rootKey = newRootKey(cc.cmd, "deriving starting key")

	return cc.cmd
}

func (c *recoverLoopInCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	if c.TxID == "" {
		return fmt.Errorf("txid is required")
	}

	if c.SwapHash == "" {
		return fmt.Errorf("swap_hash is required")
	}

	if c.LoopDbDir == "" {
		return fmt.Errorf("loop_db_dir is required")
	}

	if c.SweepAddr == "" {
		return fmt.Errorf("sweep_addr is required")
	}

	api := &btc.ExplorerAPI{BaseURL: c.APIURL}

	signer := &lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}

	// Try to fetch the swap from the database.
	store, err := loopdb.NewBoltSwapStore(c.LoopDbDir, chainParams)
	if err != nil {
		return err
	}
	defer store.Close()

	swaps, err := store.FetchLoopInSwaps()
	if err != nil {
		return err
	}

	var loopIn *loopdb.LoopIn
	for _, s := range swaps {
		if s.Hash.String() == c.SwapHash {
			loopIn = s
			break
		}
	}
	if loopIn == nil {
		return fmt.Errorf("swap not found")
	}

	fmt.Println("Loop expires at block height", loopIn.Contract.CltvExpiry)

	// Get the swaps htlc.
	htlc, err := loop.GetHtlc(
		loopIn.Hash, &loopIn.Contract.SwapContract, chainParams,
	)
	if err != nil {
		return err
	}

	// Get the destination address.
	sweepAddr, err := btcutil.DecodeAddress(c.SweepAddr, chainParams)
	if err != nil {
		return err
	}

	// Calculate the sweep fee.
	estimator := &input.TxWeightEstimator{}
	err = htlc.AddTimeoutToEstimator(estimator)
	if err != nil {
		return err
	}

	switch sweepAddr.(type) {
	case *btcutil.AddressWitnessPubKeyHash:
		estimator.AddP2WKHOutput()

	case *btcutil.AddressTaproot:
		estimator.AddP2TROutput()

	default:
		return fmt.Errorf("unsupported address type")
	}

	feeRateKWeight := chainfee.SatPerKVByte(
		1000 * c.FeeRate,
	).FeePerKWeight()
	fee := feeRateKWeight.FeeForWeight(int64(estimator.Weight()))

	txID, err := chainhash.NewHashFromStr(c.TxID)
	if err != nil {
		return err
	}

	// Get the htlc outpoint.
	htlcOutpoint := wire.OutPoint{
		Hash:  *txID,
		Index: c.Vout,
	}

	// Compose tx.
	sweepTx := wire.NewMsgTx(2)

	sweepTx.LockTime = uint32(loopIn.Contract.CltvExpiry)

	// Add HTLC input.
	sweepTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: htlcOutpoint,
		Sequence:         0,
	})

	// Add output for the destination address.
	sweepPkScript, err := txscript.PayToAddrScript(sweepAddr)
	if err != nil {
		return err
	}

	sweepTx.AddTxOut(&wire.TxOut{
		PkScript: sweepPkScript,
		Value:    int64(loopIn.Contract.AmountRequested) - int64(fee),
	})

	// If the htlc is version 2, we need to brute force the key locator, as
	// it is not stored in the database.
	var rawTx []byte
	if htlc.Version == swap.HtlcV2 {
		fmt.Println("Brute forcing key index...")
		for i := c.StartKeyIndex; i < c.StartKeyIndex+c.NumTries; i++ {
			rawTx, err = getSignedTx(
				signer, loopIn, sweepTx, htlc,
				keychain.KeyFamily(swap.KeyFamily), uint32(i),
			)
			if err == nil {
				break
			}
		}
		if rawTx == nil {
			return fmt.Errorf("failed to brute force key index, " +
				"please try again with a higher start key " +
				"index")
		}
	} else {
		rawTx, err = getSignedTx(
			signer, loopIn, sweepTx, htlc,
			loopIn.Contract.HtlcKeys.ClientScriptKeyLocator.Family,
			loopIn.Contract.HtlcKeys.ClientScriptKeyLocator.Index,
		)
		if err != nil {
			return err
		}
	}

	// Publish TX.
	if c.Publish {
		response, err := api.PublishTx(
			hex.EncodeToString(rawTx),
		)
		if err != nil {
			return err
		}
		log.Infof("Published TX %s, response: %s",
			sweepTx.TxHash().String(), response)
	} else {
		fmt.Printf("Success, we successfully created the sweep "+
			"transaction. Please publish this using any bitcoin "+
			"node:\n\n%x\n\n", rawTx)
	}

	return nil
}

func getSignedTx(signer *lnd.Signer, loopIn *loopdb.LoopIn, sweepTx *wire.MsgTx,
	htlc *swap.Htlc, keyFamily keychain.KeyFamily,
	keyIndex uint32) ([]byte, error) {

	// Create the sign descriptor.
	prevTxOut := &wire.TxOut{
		PkScript: htlc.PkScript,
		Value:    int64(loopIn.Contract.AmountRequested),
	}
	prevOutputFetcher := txscript.NewCannedPrevOutputFetcher(
		prevTxOut.PkScript, prevTxOut.Value,
	)

	signDesc := &input.SignDescriptor{
		KeyDesc: keychain.KeyDescriptor{
			KeyLocator: keychain.KeyLocator{
				Family: keyFamily,
				Index:  keyIndex,
			},
		},
		WitnessScript:     htlc.TimeoutScript(),
		HashType:          htlc.SigHash(),
		InputIndex:        0,
		PrevOutputFetcher: prevOutputFetcher,
		Output:            prevTxOut,
	}
	switch htlc.Version {
	case swap.HtlcV2:
		signDesc.SignMethod = input.WitnessV0SignMethod

	case swap.HtlcV3:
		signDesc.SignMethod = input.TaprootScriptSpendSignMethod
	}

	sig, err := signer.SignOutputRaw(sweepTx, signDesc)
	if err != nil {
		return nil, err
	}

	witness, err := htlc.GenTimeoutWitness(sig.Serialize())
	if err != nil {
		return nil, err
	}

	sweepTx.TxIn[0].Witness = witness

	rawTx, err := encodeTx(sweepTx)
	if err != nil {
		return nil, err
	}

	sigHashes := txscript.NewTxSigHashes(sweepTx, prevOutputFetcher)

	// Verify the signature. This will throw an error if the signature is
	// invalid and allows us to bruteforce the key index.
	vm, err := txscript.NewEngine(
		prevTxOut.PkScript, sweepTx, 0, txscript.StandardVerifyFlags,
		nil, sigHashes, prevTxOut.Value, prevOutputFetcher,
	)
	if err != nil {
		return nil, err
	}

	err = vm.Execute()
	if err != nil {
		return nil, err
	}

	return rawTx, nil
}

// encodeTx encodes a tx to raw bytes.
func encodeTx(tx *wire.MsgTx) ([]byte, error) {
	var buffer bytes.Buffer
	err := tx.BtcEncode(&buffer, 0, wire.WitnessEncoding)
	if err != nil {
		return nil, err
	}
	rawTx := buffer.Bytes()

	return rawTx, nil
}
