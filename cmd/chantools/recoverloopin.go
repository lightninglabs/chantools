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
	"github.com/lightninglabs/loop"
	"github.com/lightninglabs/loop/loopdb"
	"github.com/lightninglabs/loop/swap"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/spf13/cobra"
)

var (
	errSwapNotFound = fmt.Errorf("loop in swap not found")
)

type recoverLoopInCommand struct {
	TxID          string
	Vout          uint32
	SwapHash      string
	SweepAddr     string
	OutputAmt     uint64
	FeeRate       uint32
	StartKeyIndex int
	NumTries      int

	APIURL  string
	Publish bool

	LoopDbDir  string
	SqliteFile string

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
		&cc.SweepAddr, "sweepaddr", "", "address to recover the funds "+
			"to; specify '"+lnd.AddressDeriveFromWallet+"' to "+
			"derive a new address from the seed automatically",
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
	cc.cmd.Flags().Uint64Var(
		&cc.OutputAmt, "output_amt", 0, "amount of the output to sweep",
	)
	cc.cmd.Flags().StringVar(
		&cc.SqliteFile, "sqlite_file", "", "optional path to the loop "+
			"sqlite database file, if not specified, the default "+
			"location will be loaded from --loop_db_dir",
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
		store  loopdb.SwapStore
		loopIn *loopdb.LoopIn
	)

	// First check if a boltdb file exists.
	if lnrpc.FileExists(filepath.Join(c.LoopDbDir, "loop.db")) {
		store, err = loopdb.NewBoltSwapStore(c.LoopDbDir, chainParams)
		if err != nil {
			return err
		}
		defer store.Close()

		loopIn, err = findLoopInSwap(ctx, store, c.SwapHash)
		if err != nil && !errors.Is(err, errSwapNotFound) {
			return err
		}
	}

	// If the loopin is not found yet, try to fetch it from the sqlite db.
	if loopIn == nil {
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

		loopIn, err = findLoopInSwap(ctx, sqliteDb, c.SwapHash)
		if err != nil && !errors.Is(err, errSwapNotFound) {
			return err
		}
	}

	// If the loopin is still not found, return an error.
	if loopIn == nil {
		return errSwapNotFound
	}

	// If the swap is an external htlc, we require the output amount to be
	// set, as a lot of failure cases steam from the output amount being
	// wrong.
	if loopIn.Contract.ExternalHtlc && c.OutputAmt == 0 {
		return fmt.Errorf("output_amt is required for external htlc")
	}

	fmt.Println("Loop expires at block height", loopIn.Contract.CltvExpiry)

	outputValue := loopIn.Contract.AmountRequested
	if c.OutputAmt != 0 {
		outputValue = btcutil.Amount(c.OutputAmt)
	}

	// Get the swaps htlc.
	htlc, err := loop.GetHtlc(
		loopIn.Hash, &loopIn.Contract.SwapContract, chainParams,
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
	sweepTx.AddTxOut(&wire.TxOut{
		PkScript: sweepScript,
		Value:    int64(outputValue) - int64(fee),
	})

	// If the htlc is version 2, we need to brute force the key locator, as
	// it is not stored in the database.
	var rawTx []byte
	if htlc.Version == swap.HtlcV2 {
		fmt.Println("Brute forcing key index...")
		for i := c.StartKeyIndex; i < c.StartKeyIndex+c.NumTries; i++ {
			rawTx, err = getSignedTx(
				signer, sweepTx, htlc,
				keychain.KeyFamily(swap.KeyFamily), uint32(i),
				outputValue,
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
			signer, sweepTx, htlc,
			loopIn.Contract.HtlcKeys.ClientScriptKeyLocator.Family,
			loopIn.Contract.HtlcKeys.ClientScriptKeyLocator.Index,
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

func getSignedTx(signer *lnd.Signer, sweepTx *wire.MsgTx, htlc *swap.Htlc,
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

func findLoopInSwap(ctx context.Context, store loopdb.SwapStore,
	swapHash string) (*loopdb.LoopIn, error) {

	swaps, err := store.FetchLoopInSwaps(ctx)
	if err != nil {
		return nil, err
	}

	for _, s := range swaps {
		if s.Hash.String() == swapHash {
			return s, nil
		}
	}

	return nil, errSwapNotFound
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
