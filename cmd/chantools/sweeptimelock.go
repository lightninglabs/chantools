package main

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/btc"
	"github.com/guggero/chantools/dataformat"
	"github.com/lightningnetwork/lnd/input"
)

const (
	feeSatPerByte = 2
)

type sweepTimeLockCommand struct {
	RootKey     string `long:"rootkey" description:"BIP32 HD root key to use. Leave empty to prompt for lnd 24 word aezeed."`
	Publish     bool   `long:"publish" description:"Should the sweep TX be published to the chain API?"`
	SweepAddr   string `long:"sweepaddr" description:"The address the funds should be sweeped to"`
	MaxCsvLimit int    `long:"maxcsvlimit" description:"Maximum CSV limit to use. (default 2000)"`
}

func (c *sweepTimeLockCommand) Execute(_ []string) error {
	var (
		extendedKey *hdkeychain.ExtendedKey
		err error
	)

	// Check that root key is valid or fall back to console input.
	switch {
	case c.RootKey != "":
		extendedKey, err = hdkeychain.NewKeyFromString(c.RootKey)

	default:
		extendedKey, err = rootKeyFromConsole()
	}
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	// Make sure sweep addr is set.
	if c.SweepAddr == "" {
		return fmt.Errorf("sweep addr is required")
	}

	// Parse channel entries from any of the possible input files.
	entries, err := parseInputType(cfg)
	if err != nil {
		return err
	}

	// Set default value
	if c.MaxCsvLimit == 0 {
		c.MaxCsvLimit = 2000
	}
	return sweepTimeLock(
		extendedKey, cfg.ApiUrl, entries, c.SweepAddr, c.MaxCsvLimit,
		c.Publish,
	)
}

func sweepTimeLock(extendedKey *hdkeychain.ExtendedKey, apiUrl string,
	entries []*dataformat.SummaryEntry, sweepAddr string, maxCsvTimeout int,
	publish bool) error {

	// Create signer and transaction template.
	signer := &btc.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	chainApi := &btc.ExplorerApi{BaseUrl: apiUrl}

	sweepTx := wire.NewMsgTx(2)
	totalOutputValue := int64(0)
	signDescs := make([]*input.SignDescriptor, 0)

	for _, entry := range entries {
		// Skip entries that can't be swept.
		if entry.ClosingTX == nil ||
			entry.ForceClose == nil ||
			entry.ClosingTX.AllOutsSpent ||
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
			return fmt.Errorf("error parsing commit point: %v", err)
		}
		revBase, err := pubKeyFromHex(fc.RevocationBasePoint.PubKey)
		if err != nil {
			return fmt.Errorf("error parsing commit point: %v", err)
		}
		delayDesc := fc.DelayBasePoint.Desc()
		delayPrivKey, err := signer.FetchPrivKey(delayDesc)
		if err != nil {
			return fmt.Errorf("error getting private key: %v", err)
		}
		delayBase := delayPrivKey.PubKey()

		// We can't rely on the CSV delay of the channel DB to be
		// correct. But it doesn't cost us a lot to just brute force it.
		csvTimeout, script, scriptHash, err := bruteForceDelay(
			input.TweakPubKey(delayBase, commitPoint),
			input.DeriveRevocationPubkey(revBase, commitPoint),
			fc.Outs[txindex].Script, maxCsvTimeout,
		)
		if err != nil {
			log.Errorf("Could not create matching script for %s "+
				"or csv too high: %v", entry.ChannelPoint,
				err)
			continue
		}

		// Create the transaction input.
		txHash, err := chainhash.NewHashFromStr(fc.TXID)
		if err != nil {
			return fmt.Errorf("error parsing tx hash: %v", err)
		}
		sweepTx.TxIn = append(sweepTx.TxIn, &wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  *txHash,
				Index: uint32(txindex),
			},
			Sequence: input.LockTimeToSequence(
				false, uint32(csvTimeout),
			),
		})

		// Create the sign descriptor for the input.
		signDesc := &input.SignDescriptor{
			KeyDesc: *delayDesc,
			SingleTweak: input.SingleTweakBytes(
				commitPoint, delayBase,
			),
			WitnessScript: script,
			Output: &wire.TxOut{
				PkScript: scriptHash,
				Value:    int64(fc.Outs[txindex].Value),
			},
			HashType: txscript.SigHashAll,
		}
		totalOutputValue += int64(fc.Outs[txindex].Value)
		signDescs = append(signDescs, signDesc)
	}

	// Add our sweep destination output.
	sweepScript, err := getWP2PKHScript(sweepAddr)
	if err != nil {
		return err
	}
	sweepTx.TxOut = []*wire.TxOut{{
		Value:    totalOutputValue,
		PkScript: sweepScript,
	}}

	// Very naive fee estimation algorithm: Sign a first time as if we would
	// send the whole amount with zero fee, just to estimate how big the
	// transaction would get in bytes. Then adjust the fee and sign again.
	sigHashes := txscript.NewTxSigHashes(sweepTx)
	for idx, desc := range signDescs {
		desc.SigHashes = sigHashes
		desc.InputIndex = idx
		witness, err := input.CommitSpendTimeout(signer, desc, sweepTx)
		if err != nil {
			return err
		}
		sweepTx.TxIn[idx].Witness = witness
	}

	// Calculate a fee. This won't be very accurate so the feeSatPerByte
	// should at least be 2 to not risk falling below the 1 sat/byte limit.
	size := sweepTx.SerializeSize()
	fee := int64(size * feeSatPerByte)
	sweepTx.TxOut[0].Value = totalOutputValue - fee

	// Sign again after output fixing.
	sigHashes = txscript.NewTxSigHashes(sweepTx)
	for idx, desc := range signDescs {
		desc.SigHashes = sigHashes
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
	log.Infof("Fee %d sats of %d total amount (for size %d)",
		fee, totalOutputValue, sweepTx.SerializeSize())

	// Publish TX.
	if publish {
		response, err := chainApi.PublishTx(
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
		return nil, fmt.Errorf("error hex decoding pub key: %v", err)
	}
	return btcec.ParsePubKey(
		pointBytes, btcec.S256(),
	)
}

func getWP2PKHScript(addr string) ([]byte, error) {
	targetPubKeyHash, err := parseAddr(addr)
	if err != nil {
		return nil, err
	}
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_0)
	builder.AddData(targetPubKeyHash)

	return builder.Script()
}

func bruteForceDelay(delayPubkey, revocationPubkey *btcec.PublicKey,
	targetScriptHex string, maxCsvTimeout int) (int32, []byte, []byte,
	error) {

	targetScript, err := hex.DecodeString(targetScriptHex)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("error parsing target script: "+
			"%v", err)
	}
	if len(targetScript) != 34 {
		return 0, nil, nil, fmt.Errorf("invalid target script: %s",
			targetScriptHex)
	}
	for i := 0; i <= maxCsvTimeout; i++ {
		s, err := input.CommitScriptToSelf(
			uint32(i), delayPubkey, revocationPubkey,
		)
		if err != nil {
			return 0, nil, nil, fmt.Errorf("error creating "+
				"script: %v", err)
		}
		sh, err := input.WitnessScriptHash(s)
		if err != nil {
			return 0, nil, nil, fmt.Errorf("error hashing script: "+
				"%v", err)
		}
		if bytes.Equal(targetScript[0:8], sh[0:8]) {
			return int32(i), s, sh, nil
		}
	}
	return 0, nil, nil, fmt.Errorf("csv timeout not found for target "+
		"script %s", targetScriptHex)
}
