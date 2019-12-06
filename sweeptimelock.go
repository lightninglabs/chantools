package chantools

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
)

const (
	maxCsvTimeout = 15000
	feeSatPerByte = 3
)

func sweepTimeLock(cfg *config, entries []*SummaryEntry, sweepAddr string,
	publish bool) error {

	extendedKey, err := hdkeychain.NewKeyFromString(cfg.RootKey)
	if err != nil {
		return err
	}
	signer := &signer{extendedKey: extendedKey}
	sweepTx := wire.NewMsgTx(2)
	entryIndex := 0
	value := int64(0)
	signDescs := make([]*input.SignDescriptor, 0)

	for _, entry := range entries {
		if entry.ClosingTX == nil || entry.ForceClose == nil ||
			entry.ClosingTX.AllOutsSpent || entry.LocalBalance == 0 {

			log.Infof("Not sweeping %s, info missing or all spent",
				entry.ChannelPoint)
			continue
		}

		fc := entry.ForceClose

		txindex := -1
		if len(fc.Outs) == 1 {
			txindex = 0
			if fc.Outs[0].Value != uint64(entry.LocalBalance) {
				log.Errorf("Potential value mismatch! %d vs %d (%s)",
					fc.Outs[0].Value, entry.LocalBalance,
					entry.ChannelPoint)
			}
		} else {
			for idx, out := range fc.Outs {
				if out.Value == uint64(entry.LocalBalance) {
					txindex = idx
				}
			}
		}

		if txindex == -1 {
			log.Errorf("Could not find sweep output for chan %s",
				entry.ChannelPoint)
			continue
		}
		txHash, err := chainhash.NewHashFromStr(fc.TXID)
		if err != nil {
			return fmt.Errorf("error parsing tx hash: %v", err)
		}

		commitPointBytes, err := hex.DecodeString(fc.CommitPoint)
		if err != nil {
			return fmt.Errorf("error parsing commit point: %v", err)
		}
		commitPoint, err := btcec.ParsePubKey(commitPointBytes, btcec.S256())
		if err != nil {
			return fmt.Errorf("error parsing commit point: %v", err)
		}
		revPointBytes, err := hex.DecodeString(fc.RevocationBasepoint.Pubkey)
		if err != nil {
			return fmt.Errorf("error parsing commit point: %v", err)
		}
		revPoint, err := btcec.ParsePubKey(revPointBytes, btcec.S256())
		if err != nil {
			return fmt.Errorf("error parsing commit point: %v", err)
		}

		delayKeyDesc := &keychain.KeyDescriptor{
			KeyLocator: keychain.KeyLocator{
				Family: keychain.KeyFamily(fc.DelayBasepoint.Family),
				Index:  fc.DelayBasepoint.Index,
			},
		}
		delayPrivkey, err := signer.fetchPrivKey(delayKeyDesc)
		if err != nil {
			return fmt.Errorf("error getting private key: %v", err)
		}

		delayKey := input.TweakPubKey(delayPrivkey.PubKey(), commitPoint)
		revocationKey := input.DeriveRevocationPubkey(
			revPoint, commitPoint,
		)

		var (
			csvTimeout  = int32(-1)
			script     []byte
			scriptHash []byte
		)
		targetScript, err := hex.DecodeString(fc.Outs[txindex].Script)
		if err != nil {
			return fmt.Errorf("error parsing target script: %v", err)
		}
		if len(targetScript) != 34 {
			log.Errorf("invalid target script: %x", targetScript)
			continue
		}
		for i := 0; csvTimeout == -1 && i < maxCsvTimeout; i++ {
			s, err := input.CommitScriptToSelf(
				uint32(i), delayKey, revocationKey,
			)
			if err != nil {
				return fmt.Errorf("error creating script: %v", err)
			}
			sh, err := input.WitnessScriptHash(s)
			if err != nil {
				return fmt.Errorf("error hashing script: %v", err)
			}
			if bytes.Equal(targetScript[0:8], sh[0:8]) {
				csvTimeout = int32(i)
				script = s
				scriptHash = sh
			}
		}
		if csvTimeout == -1 || len(script) == 0 {
			log.Errorf("Could not create matching script for %s " +
				"or csv too high: %d", entry.ChannelPoint,
				csvTimeout)
			continue
		}

		sweepTx.TxIn = append(sweepTx.TxIn, &wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  *txHash,
				Index: uint32(txindex),
			},
			SignatureScript: nil,
			Witness:         nil,
			Sequence:        input.LockTimeToSequence(
				false, uint32(csvTimeout),
			),
		})

		singleTweak := input.SingleTweakBytes(
			commitPoint, delayPrivkey.PubKey(),
		)

		signDesc := &input.SignDescriptor{
			KeyDesc:       *delayKeyDesc,
			SingleTweak:   singleTweak,
			WitnessScript: script,
			Output: &wire.TxOut{
				PkScript: scriptHash,
				Value: int64(fc.Outs[txindex].Value),
			},
			HashType:   txscript.SigHashAll,
			InputIndex: entryIndex,
		}
		value += int64(fc.Outs[txindex].Value)
		signDescs = append(signDescs, signDesc)

		entryIndex++
	}
	
	if len(signDescs) != len(sweepTx.TxIn) {
		return fmt.Errorf("length mismatch")
	}

	sweepScript, err := pkhScript(sweepAddr)
	if err != nil {
		return err
	}
	sweepTx.TxOut = []*wire.TxOut{{
		Value:    value,
		PkScript: sweepScript,
	}}
	
	sigHashes := txscript.NewTxSigHashes(sweepTx)
	for idx, desc := range signDescs {
		desc.SigHashes = sigHashes
		witness, err := input.CommitSpendTimeout(signer, desc, sweepTx)
		if err != nil {
			return err
		}
		sweepTx.TxIn[idx].Witness = witness
	}
	
	size := sweepTx.SerializeSize()
	fee := int64(size*feeSatPerByte)
	sweepTx.TxOut[0].Value = value - fee

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
		fee, value, sweepTx.SerializeSize())
	log.Infof("Transaction: %x", buf.Bytes())

	return nil
}

func pkhScript(addr string) ([]byte, error) {
	targetPubKeyHash, err := parseAddr(addr)
	if err != nil {
		return nil, err
	}
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_0)
	builder.AddData(targetPubKeyHash)

	return builder.Script()
}
