package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightninglabs/loop/loopdb"
	"github.com/lightninglabs/loop/swap"
	"github.com/lightninglabs/loop/utils"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/spf13/cobra"
)

var (
	errLoopOutSwapNotFound = errors.New("loop out swap not found")
)

type recoverLoopOutCommand struct {
	TxID               string
	Vout               uint32
	SwapHash           string
	SweepAddr          string
	OutputAmt          uint64
	FeeRate            uint32
	CurrentBlockHeight uint32
	StartKeyIndex      int
	NumTries           int

	APIURL  string
	Publish bool

	LoopDbDir  string
	SqliteFile string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newRecoverLoopOutCommand() *cobra.Command {
	cc := &recoverLoopOutCommand{}
	cc.cmd = &cobra.Command{
		Use:   "recoverloopout",
		Short: "Recover a loop out swap that the loop daemon is not able to sweep",
		Example: `chantools recoverloopout \
	--loop_db_dir /path/to/loop/db/dir \
	--txid abcdef01234... \
	--vout ... \
	--output_amt ... \
	--swap_hash abcdef01234... \
	--current_block_height ... \
	--sweepaddr bc1pxxxxxxx \
	--feerate sat_per_vbyte`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.TxID, "txid", "", "transaction id of the on-chain transaction that created the HTLC",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.Vout, "vout", 0, "output index of the on-chain transaction that created the HTLC",
	)
	cc.cmd.Flags().StringVar(
		&cc.SwapHash, "swap_hash", "", "swap hash of the loop out swap",
	)
	cc.cmd.Flags().StringVar(
		&cc.LoopDbDir, "loop_db_dir", "", "path to the loop database directory, where the loop.db file is located",
	)
	cc.cmd.Flags().StringVar(
		&cc.SweepAddr, "sweepaddr", "", "address to recover the funds to; specify '"+lnd.AddressDeriveFromWallet+"' to derive a new address from the seed automatically",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.FeeRate, "feerate", 0, "fee rate to use for the sweep transaction in sat/vByte",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.CurrentBlockHeight, "current_block_height", 0, "current block height",
	)
	cc.cmd.Flags().IntVar(
		&cc.NumTries, "num_tries", 1000, "number of tries to try to find the correct key index",
	)
	cc.cmd.Flags().IntVar(
		&cc.StartKeyIndex, "start_key_index", 0, "start key index to try to find the correct key index",
	)
	cc.cmd.Flags().StringVar(
		&cc.APIURL, "apiurl", defaultAPIURL, "API URL to use (must be esplora compatible)",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Publish, "publish", false, "publish sweep TX to the chain API instead of just printing the TX",
	)
	cc.cmd.Flags().Uint64Var(
		&cc.OutputAmt, "output_amt", 0, "amount of the output to sweep",
	)
	cc.cmd.Flags().StringVar(
		&cc.SqliteFile, "sqlite_file", "", "optional path to the loop sqlite database file, if not specified, the default location will be loaded from --loop_db_dir",
	)

	cc.rootKey = newRootKey(cc.cmd, "deriving starting key")

	return cc.cmd
}

func (c *recoverLoopOutCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	if c.TxID == "" {
		return errors.New("txid is required")
	}

	if c.SwapHash == "" {
		return errors.New("swap_hash is required")
	}

	if c.LoopDbDir == "" {
		return errors.New("loop_db_dir is required")
	}

	err = lnd.CheckAddress(
		c.SweepAddr, chainParams, true, "sweep", lnd.AddrTypeP2WKH,
		lnd.AddrTypeP2TR,
	)
	if err != nil {
		return err
	}

	api := newExplorerAPI(c.APIURL)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	signer := &lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}

	// Try to fetch the swap from the boltdb.
	var (
		store   loopdb.SwapStore
		loopOut *loopdb.LoopOut
	)

	// First, check if a boltdb file exists.
	if lnrpc.FileExists(filepath.Join(c.LoopDbDir, "loop.db")) {
		store, err = loopdb.NewBoltSwapStore(c.LoopDbDir, chainParams)
		if err != nil {
			return err
		}
		defer store.Close()

		loopOut, err = findLoopOutSwap(ctx, store, c.SwapHash)
		if err != nil && !errors.Is(err, errLoopOutSwapNotFound) {
			return err
		}
	}

	// If the loopout is not found yet, try to fetch it from the sqlite db.
	if loopOut == nil {
		if c.SqliteFile == "" {
			c.SqliteFile = filepath.Join(
				c.LoopDbDir, "loop_sqlite.db",
			)
		}

		sqliteDb, err := loopdb.NewSqliteStore(
			&loopdb.SqliteConfig{
				DatabaseFileName: c.SqliteFile,
				SkipMigrations:   true,
			}, chainParams,
		)
		if err != nil {
			return err
		}
		defer sqliteDb.Close()

		loopOut, err = findLoopOutSwap(ctx, sqliteDb, c.SwapHash)
		if err != nil && !errors.Is(err, errLoopOutSwapNotFound) {
			return err
		}
	}

	// If the loopout is still not found, return an error.
	if loopOut == nil {
		return errLoopOutSwapNotFound
	}

	fmt.Println("Loop expires at block height", loopOut.Contract.CltvExpiry)

	outputValue := loopOut.Contract.AmountRequested
	if c.OutputAmt != 0 {
		outputValue = btcutil.Amount(c.OutputAmt)
	}

	// Get the swaps htlc.
	htlc, err := utils.GetHtlc(
		loopOut.Hash, &loopOut.Contract.SwapContract, chainParams,
	)
	if err != nil {
		return err
	}

	// Get the destination address.
	var estimator input.TxWeightEstimator
	sweepScript, err := lnd.PrepareWalletAddress(
		c.SweepAddr, chainParams, &estimator, extendedKey, "sweep",
	)
	if err != nil {
		return err
	}

	// Calculate the sweep fee.
	err = htlc.AddTimeoutToEstimator(&estimator)
	if err != nil {
		return err
	}

	feeRateKWeight := chainfee.SatPerKVByte(
		1000 * c.FeeRate,
	).FeePerKWeight()
	fee := feeRateKWeight.FeeForWeight(estimator.Weight())

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

	sweepTx.LockTime = uint32(loopOut.Contract.CltvExpiry)

	// Add HTLC input.
	sweepTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: htlcOutpoint,
		Sequence:         htlc.SuccessSequence(),
	})

	// Add output for the destination address.
	sweepTx.AddTxOut(&wire.TxOut{
		PkScript: sweepScript,
		Value:    int64(outputValue) - int64(fee),
	})

	// If the htlc is version 2, we need to brute force the key locator, as
	// it is not stored in the database.
	var rawTx []byte
	if htlc.Version == swap.HtlcV2 {
		fmt.Println("Brute forcing key index...")
		fmt.Println("V2 key index:")
		for i := c.StartKeyIndex; i < c.StartKeyIndex+c.NumTries; i++ {
			rawTx, err = getSignedTxLoopOut(
				signer, sweepTx, htlc,
				loopOut.Contract.Preimage,
				keychain.KeyFamily(swap.KeyFamily), uint32(i),
				outputValue,
			)
			if err == nil {
				break
			}
		}
		if rawTx == nil {
			return errors.New("failed to brute force key index, " +
				"please try again with a higher start key " +
				"index")
		}
	} else {
		fmt.Println("V3 key index:")
		rawTx, err = getSignedTxLoopOut(
			signer, sweepTx, htlc, loopOut.Contract.Preimage,
			loopOut.Contract.HtlcKeys.ClientScriptKeyLocator.Family,
			loopOut.Contract.HtlcKeys.ClientScriptKeyLocator.Index,
			outputValue,
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

func getSignedTxLoopOut(signer *lnd.Signer, sweepTx *wire.MsgTx,
	htlc *swap.Htlc, preimage lntypes.Preimage,
	keyFamily keychain.KeyFamily, keyIndex uint32,
	outputValue btcutil.Amount) ([]byte, error) {

	// Create the sign descriptor.
	prevTxOut := &wire.TxOut{
		PkScript: htlc.PkScript,
		Value:    int64(outputValue),
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
		WitnessScript:     htlc.SuccessScript(),
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

	witness, err := htlc.GenSuccessWitness(sig.Serialize(), preimage)
	if err != nil {
		return nil, err
	}

	sweepTx.TxIn[0].Witness = witness

	rawTx, err := encodeTxLoopOut(sweepTx)
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
	fmt.Printf("output value: %v\n", outputValue)
	fmt.Printf("htlc: %#v\n", htlc)

	return rawTx, nil
}

// encodeTxLoopOut encodes a tx to raw bytes.
func encodeTxLoopOut(tx *wire.MsgTx) ([]byte, error) {
	var buffer bytes.Buffer
	err := tx.BtcEncode(&buffer, 0, wire.WitnessEncoding)
	if err != nil {
		return nil, err
	}
	rawTx := buffer.Bytes()

	return rawTx, nil
}

func findLoopOutSwap(ctx context.Context, store loopdb.SwapStore,
	swapHash string) (*loopdb.LoopOut, error) {

	swaps, err := store.FetchLoopOutSwaps(ctx)
	if err != nil {
		return nil, err
	}

	for _, s := range swaps {
		if s.Hash.String() == swapHash {
			return s, nil
		}
	}

	return nil, errLoopOutSwapNotFound
}
