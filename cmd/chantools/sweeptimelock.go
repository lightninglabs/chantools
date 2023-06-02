package main

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/dataformat"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/spf13/cobra"
)

const (
	defaultFeeSatPerVByte = 30
	defaultCsvLimit       = 2016
)

type sweepTimeLockCommand struct {
	APIURL      string
	Publish     bool
	SweepAddr   string
	MaxCsvLimit uint16
	FeeRate     uint32

	rootKey *rootKey
	inputs  *inputFlags
	cmd     *cobra.Command
}

func newSweepTimeLockCommand() *cobra.Command {
	cc := &sweepTimeLockCommand{}
	cc.cmd = &cobra.Command{
		Use: "sweeptimelock",
		Short: "Sweep the force-closed state after the time lock has " +
			"expired",
		Long: `Use this command to sweep the funds from channels that
you force-closed with the forceclose command. You **MUST** use the result file
that was created with the forceclose command, otherwise it won't work. You also
have to wait until the highest time lock (can be up to 2016 blocks which is more
than two weeks) of all the channels has passed. If you only want to sweep
channels that have the default CSV limit of 1 day, you can set the --maxcsvlimit
parameter to 144.`,
		Example: `chantools sweeptimelock \
	--fromsummary results/forceclose-xxxx-yyyy.json \
	--sweepaddr bc1q..... \
	--feerate 10 \
  	--publish`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.APIURL, "apiurl", defaultAPIURL, "API URL to use (must "+
			"be esplora compatible)",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Publish, "publish", false, "publish sweep TX to the chain "+
			"API instead of just printing the TX",
	)
	cc.cmd.Flags().StringVar(
		&cc.SweepAddr, "sweepaddr", "", "address to sweep the funds to",
	)
	cc.cmd.Flags().Uint16Var(
		&cc.MaxCsvLimit, "maxcsvlimit", defaultCsvLimit, "maximum CSV "+
			"limit to use",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.FeeRate, "feerate", defaultFeeSatPerVByte, "fee rate to "+
			"use for the sweep transaction in sat/vByte",
	)

	cc.rootKey = newRootKey(cc.cmd, "deriving keys")
	cc.inputs = newInputFlags(cc.cmd)

	return cc.cmd
}

func (c *sweepTimeLockCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	// Make sure sweep addr is set.
	if c.SweepAddr == "" {
		return fmt.Errorf("sweep addr is required")
	}

	// Parse channel entries from any of the possible input files.
	entries, err := c.inputs.parseInputType()
	if err != nil {
		return err
	}

	// Set default values.
	if c.MaxCsvLimit == 0 {
		c.MaxCsvLimit = defaultCsvLimit
	}
	if c.FeeRate == 0 {
		c.FeeRate = defaultFeeSatPerVByte
	}
	return sweepTimeLockFromSummary(
		extendedKey, c.APIURL, entries, c.SweepAddr, c.MaxCsvLimit,
		c.Publish, c.FeeRate,
	)
}

type sweepTarget struct {
	channelPoint        string
	txid                chainhash.Hash
	index               uint32
	lockScript          []byte
	value               int64
	commitPoint         *btcec.PublicKey
	revocationBasePoint *btcec.PublicKey
	delayBasePointDesc  *keychain.KeyDescriptor
}

func sweepTimeLockFromSummary(extendedKey *hdkeychain.ExtendedKey, apiURL string,
	entries []*dataformat.SummaryEntry, sweepAddr string,
	maxCsvTimeout uint16, publish bool, feeRate uint32) error {

	targets := make([]*sweepTarget, 0, len(entries))
	for _, entry := range entries {
		// Skip entries that can't be swept.
		if entry.ForceClose == nil ||
			(entry.ClosingTX != nil && entry.ClosingTX.AllOutsSpent) ||
			entry.LocalBalance == 0 {

			log.Infof("Not sweeping %s, info missing or all spent",
				entry.ChannelPoint)

			continue
		}

		fc := entry.ForceClose

		// Find index of sweepable output of commitment TX.
		txindex := -1
		if len(fc.Outs) == 1 {
			txindex = 0
			if fc.Outs[0].Value != entry.LocalBalance {
				log.Errorf("Potential value mismatch! %d vs "+
					"%d (%s)",
					fc.Outs[0].Value, entry.LocalBalance,
					entry.ChannelPoint)
			}
		} else {
			for idx, out := range fc.Outs {
				if out.Value == entry.LocalBalance {
					txindex = idx
				}
			}
		}
		if txindex == -1 {
			log.Errorf("Could not find sweep output for chan %s",
				entry.ChannelPoint)
			continue
		}

		// Prepare sweep script parameters.
		commitPoint, err := pubKeyFromHex(fc.CommitPoint)
		if err != nil {
			return fmt.Errorf("error parsing commit point: %w", err)
		}
		revBase, err := pubKeyFromHex(fc.RevocationBasePoint.PubKey)
		if err != nil {
			return fmt.Errorf("error parsing revocation base "+
				"point: %w", err)
		}
		delayDesc, err := fc.DelayBasePoint.Desc()
		if err != nil {
			return fmt.Errorf("error parsing delay base point: %w",
				err)
		}

		lockScript, err := hex.DecodeString(fc.Outs[txindex].Script)
		if err != nil {
			return fmt.Errorf("error parsing target script: %w",
				err)
		}

		// Create the transaction input.
		txHash, err := chainhash.NewHashFromStr(fc.TXID)
		if err != nil {
			return fmt.Errorf("error parsing tx hash: %w", err)
		}

		targets = append(targets, &sweepTarget{
			channelPoint:        entry.ChannelPoint,
			txid:                *txHash,
			index:               uint32(txindex),
			lockScript:          lockScript,
			value:               int64(fc.Outs[txindex].Value),
			commitPoint:         commitPoint,
			revocationBasePoint: revBase,
			delayBasePointDesc:  delayDesc,
		})
	}

	return sweepTimeLock(
		extendedKey, apiURL, targets, sweepAddr, maxCsvTimeout, publish,
		feeRate,
	)
}

func sweepTimeLock(extendedKey *hdkeychain.ExtendedKey, apiURL string,
	targets []*sweepTarget, sweepAddr string, maxCsvTimeout uint16,
	publish bool, feeRate uint32) error {

	// Create signer and transaction template.
	signer := &lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	api := &btc.ExplorerAPI{BaseURL: apiURL}

	var (
		sweepTx          = wire.NewMsgTx(2)
		totalOutputValue = int64(0)
		signDescs        = make([]*input.SignDescriptor, 0)
		prevOutFetcher   = txscript.NewMultiPrevOutFetcher(nil)
		estimator        input.TxWeightEstimator
	)
	for _, target := range targets {
		// We can't rely on the CSV delay of the channel DB to be
		// correct. But it doesn't cost us a lot to just brute force it.
		csvTimeout, script, scriptHash, err := bruteForceDelay(
			input.TweakPubKey(
				target.delayBasePointDesc.PubKey,
				target.commitPoint,
			), input.DeriveRevocationPubkey(
				target.revocationBasePoint,
				target.commitPoint,
			), target.lockScript, maxCsvTimeout,
		)
		if err != nil {
			log.Errorf("Could not create matching script for %s "+
				"or csv too high: %w", target.channelPoint, err)
			continue
		}

		// Create the transaction input.
		prevOutPoint := wire.OutPoint{
			Hash:  target.txid,
			Index: target.index,
		}
		prevTxOut := &wire.TxOut{
			PkScript: scriptHash,
			Value:    target.value,
		}
		prevOutFetcher.AddPrevOut(prevOutPoint, prevTxOut)
		sweepTx.TxIn = append(sweepTx.TxIn, &wire.TxIn{
			PreviousOutPoint: prevOutPoint,
			Sequence: input.LockTimeToSequence(
				false, uint32(csvTimeout),
			),
		})

		// Create the sign descriptor for the input.
		signDesc := &input.SignDescriptor{
			KeyDesc: *target.delayBasePointDesc,
			SingleTweak: input.SingleTweakBytes(
				target.commitPoint,
				target.delayBasePointDesc.PubKey,
			),
			WitnessScript:     script,
			Output:            prevTxOut,
			HashType:          txscript.SigHashAll,
			PrevOutputFetcher: prevOutFetcher,
		}
		totalOutputValue += target.value
		signDescs = append(signDescs, signDesc)

		// Account for the input weight.
		estimator.AddWitnessInput(input.ToLocalTimeoutWitnessSize)
	}

	// Add our sweep destination output.
	sweepScript, err := lnd.GetP2WPKHScript(sweepAddr, chainParams)
	if err != nil {
		return err
	}
	estimator.AddP2WKHOutput()

	// Calculate the fee based on the given fee rate and our weight
	// estimation.
	feeRateKWeight := chainfee.SatPerKVByte(1000 * feeRate).FeePerKWeight()
	totalFee := feeRateKWeight.FeeForWeight(int64(estimator.Weight()))

	log.Infof("Fee %d sats of %d total amount (estimated weight %d)",
		totalFee, totalOutputValue, estimator.Weight())

	sweepTx.TxOut = []*wire.TxOut{{
		Value:    totalOutputValue - int64(totalFee),
		PkScript: sweepScript,
	}}

	// Sign the transaction now.
	sigHashes := txscript.NewTxSigHashes(sweepTx, prevOutFetcher)
	for idx, desc := range signDescs {
		desc.SigHashes = sigHashes
		desc.InputIndex = idx
		witness, err := input.CommitSpendTimeout(signer, desc, sweepTx)
		if err != nil {
			return err
		}
		sweepTx.TxIn[idx].Witness = witness
	}

	var buf bytes.Buffer
	err = sweepTx.Serialize(&buf)
	if err != nil {
		return err
	}

	// Publish TX.
	if publish {
		response, err := api.PublishTx(
			hex.EncodeToString(buf.Bytes()),
		)
		if err != nil {
			return err
		}
		log.Infof("Published TX %s, response: %s",
			sweepTx.TxHash().String(), response)
	}

	log.Infof("Transaction: %x", buf.Bytes())
	return nil
}

func pubKeyFromHex(pubKeyHex string) (*btcec.PublicKey, error) {
	pointBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error hex decoding pub key: %w", err)
	}
	return btcec.ParsePubKey(pointBytes)
}

func bruteForceDelay(delayPubkey, revocationPubkey *btcec.PublicKey,
	targetScript []byte, maxCsvTimeout uint16) (int32, []byte, []byte,
	error) {

	if len(targetScript) != 34 {
		return 0, nil, nil, fmt.Errorf("invalid target script: %s",
			targetScript)
	}
	for i := uint16(0); i <= maxCsvTimeout; i++ {
		s, err := input.CommitScriptToSelf(
			uint32(i), delayPubkey, revocationPubkey,
		)
		if err != nil {
			return 0, nil, nil, fmt.Errorf("error creating "+
				"script: %w", err)
		}
		sh, err := input.WitnessScriptHash(s)
		if err != nil {
			return 0, nil, nil, fmt.Errorf("error hashing script: "+
				"%w", err)
		}
		if bytes.Equal(targetScript[0:8], sh[0:8]) {
			return int32(i), s, sh, nil
		}
	}
	return 0, nil, nil, fmt.Errorf("csv timeout not found for target "+
		"script %s", targetScript)
}
