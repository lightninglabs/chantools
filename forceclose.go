package chantools

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
)

func forceCloseChannels(cfg *config, entries []*SummaryEntry,
	chanDb *channeldb.DB, publish bool) error {

	channels, err := chanDb.FetchAllChannels()
	if err != nil {
		return err
	}
	
	chainApi := &chainApi{baseUrl:cfg.ApiUrl}

	extendedKey, err := hdkeychain.NewKeyFromString(cfg.RootKey)
	if err != nil {
		return err
	}
	signer := &signer{extendedKey: extendedKey}

	// Go through all channels in the DB, find the still open ones and
	// publish their local commitment TX.
	for _, channel := range channels {
		channelPoint := channel.FundingOutpoint.String()
		var channelEntry *SummaryEntry
		for _, entry := range entries {
			if entry.ChannelPoint == channelPoint {
				channelEntry = entry
			}
		}

		// Don't try anything with closed channels.
		if channelEntry == nil || channelEntry.ClosingTX != nil {
			continue
		}

		localCommit := channel.LocalCommitment
		localCommitTx := localCommit.CommitTx
		if localCommitTx == nil {
			log.Errorf("Cannot force-close, no local commit TX for "+
				"channel %s", channelEntry.ChannelPoint)
			continue
		}

		// Create signed transaction.
		lc := &LightningChannel{
			localChanCfg:  channel.LocalChanCfg,
			remoteChanCfg: channel.RemoteChanCfg,
			channelState:  channel,
			txSigner:      signer,
		}
		err := lc.createSignDesc()
		if err != nil {
			return err
		}

		// Serialize transaction.
		signedTx, err := lc.getSignedCommitTx()
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		err = signedTx.Serialize(io.Writer(&buf))
		if err != nil {
			return err
		}
		hash := signedTx.TxHash()
		serialized := hex.EncodeToString(buf.Bytes())

		// Calculate commit point.
		basepoint := channel.LocalChanCfg.DelayBasePoint
		revocationPreimage, err := channel.RevocationProducer.AtIndex(
			localCommit.CommitHeight,
		)
		if err != nil {
			return err
		}
		point := input.ComputeCommitmentPoint(revocationPreimage[:])
		channelEntry.ForceClose = &ForceClose{
			TXID:       hash.String(),
			Serialized: serialized,
			DelayBasepoint: &Basepoint{
				Family: uint16(basepoint.Family),
				Index:  basepoint.Index,
			},
			CommitPoint: hex.EncodeToString(
				point.SerializeCompressed(),
			),
			Outs: make([]*Out, len(localCommitTx.TxOut)),
		}
		for idx, out := range localCommitTx.TxOut {
			script, err := txscript.DisasmString(out.PkScript)
			if err != nil {
				return err
			}
			channelEntry.ForceClose.Outs[idx] = &Out{
				Script:    hex.EncodeToString(out.PkScript),
				ScriptAsm: script,
				Value:     uint64(out.Value),
			}
		}
		
		// Publish TX.
		if publish {
			response, err := chainApi.PublishTx(serialized)
			if err != nil {
				return err
			}
			log.Infof("Published TX %s, response: %s", hash.String(),
				response)
		}
	}

	summaryBytes, err := json.MarshalIndent(&SummaryEntryFile{
		Channels: entries,
	}, "", " ")
	if err != nil {
		return err
	}
	fileName := fmt.Sprintf("results/forceclose-%s.json",
		time.Now().Format("2006-01-02-15-04-05"))
	log.Infof("Writing result to %s", fileName)
	return ioutil.WriteFile(fileName, summaryBytes, 0644)
}

type signer struct {
	extendedKey *hdkeychain.ExtendedKey
}

func (s *signer) SignOutputRaw(tx *wire.MsgTx,
	signDesc *input.SignDescriptor) ([]byte, error) {
	witnessScript := signDesc.WitnessScript

	// First attempt to fetch the private key which corresponds to the
	// specified public key.
	privKey, err := s.fetchPrivKey(&signDesc.KeyDesc)
	if err != nil {
		return nil, err
	}

	amt := signDesc.Output.Value
	sig, err := txscript.RawTxInWitnessSignature(
		tx, signDesc.SigHashes, signDesc.InputIndex, amt,
		witnessScript, signDesc.HashType, privKey,
	)
	if err != nil {
		return nil, err
	}

	// Chop off the sighash flag at the end of the signature.
	return sig[:len(sig)-1], nil
}

func (s *signer) fetchPrivKey(descriptor *keychain.KeyDescriptor) (
	*btcec.PrivateKey, error) {

	key, err := deriveChildren(s.extendedKey, []uint32{
		hardenedKeyStart + uint32(keychain.BIP0043Purpose),
		hardenedKeyStart + chainParams.HDCoinType,
		hardenedKeyStart + uint32(descriptor.Family),
		0,
		descriptor.Index,
	})
	if err != nil {
		return nil, err
	}
	return key.ECPrivKey()
}

type LightningChannel struct {
	localChanCfg  channeldb.ChannelConfig
	remoteChanCfg channeldb.ChannelConfig
	signDesc      *input.SignDescriptor
	channelState  *channeldb.OpenChannel
	txSigner      *signer
}

// createSignDesc derives the SignDescriptor for commitment transactions from
// other fields on the LightningChannel.
func (lc *LightningChannel) createSignDesc() error {
	localKey := lc.localChanCfg.MultiSigKey.PubKey.SerializeCompressed()
	remoteKey := lc.remoteChanCfg.MultiSigKey.PubKey.SerializeCompressed()

	multiSigScript, err := input.GenMultiSigScript(localKey, remoteKey)
	if err != nil {
		return err
	}

	fundingPkScript, err := input.WitnessScriptHash(multiSigScript)
	if err != nil {
		return err
	}
	lc.signDesc = &input.SignDescriptor{
		KeyDesc:       lc.localChanCfg.MultiSigKey,
		WitnessScript: multiSigScript,
		Output: &wire.TxOut{
			PkScript: fundingPkScript,
			Value:    int64(lc.channelState.Capacity),
		},
		HashType:   txscript.SigHashAll,
		InputIndex: 0,
	}

	return nil
}

// getSignedCommitTx function take the latest commitment transaction and
// populate it with witness data.
func (lc *LightningChannel) getSignedCommitTx() (*wire.MsgTx, error) {
	// Fetch the current commitment transaction, along with their signature
	// for the transaction.
	localCommit := lc.channelState.LocalCommitment
	commitTx := localCommit.CommitTx.Copy()
	theirSig := append(localCommit.CommitSig, byte(txscript.SigHashAll))

	// With this, we then generate the full witness so the caller can
	// broadcast a fully signed transaction.
	lc.signDesc.SigHashes = txscript.NewTxSigHashes(commitTx)
	ourSigRaw, err := lc.txSigner.SignOutputRaw(commitTx, lc.signDesc)
	if err != nil {
		return nil, err
	}

	ourSig := append(ourSigRaw, byte(txscript.SigHashAll))

	// With the final signature generated, create the witness stack
	// required to spend from the multi-sig output.
	ourKey := lc.localChanCfg.MultiSigKey.PubKey.SerializeCompressed()
	theirKey := lc.remoteChanCfg.MultiSigKey.PubKey.SerializeCompressed()

	commitTx.TxIn[0].Witness = input.SpendMultiSig(
		lc.signDesc.WitnessScript, ourKey,
		ourSig, theirKey, theirSig,
	)

	return commitTx, nil
}
