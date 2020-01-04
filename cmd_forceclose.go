package chantools

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/chain"
	"github.com/guggero/chantools/dataformat"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
)

func forceCloseChannels(extendedKey *hdkeychain.ExtendedKey,
	entries []*dataformat.SummaryEntry, chanDb *channeldb.DB,
	publish bool) error {

	channels, err := chanDb.FetchAllChannels()
	if err != nil {
		return err
	}
	chainApi := &chain.Api{BaseUrl: cfg.ApiUrl}
	signer := &signer{extendedKey: extendedKey}

	// Go through all channels in the DB, find the still open ones and
	// publish their local commitment TX.
	for _, channel := range channels {
		channelPoint := channel.FundingOutpoint.String()
		var channelEntry *dataformat.SummaryEntry
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
			log.Errorf("Cannot force-close, no local commit TX "+
				"for channel %s", channelEntry.ChannelPoint)
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
		revpoint := channel.RemoteChanCfg.RevocationBasePoint
		revocationPreimage, err := channel.RevocationProducer.AtIndex(
			localCommit.CommitHeight,
		)
		if err != nil {
			return err
		}
		point := input.ComputeCommitmentPoint(revocationPreimage[:])

		// Store all information that we collected into the channel
		// entry file so we don't need to use the channel.db file for
		// the next step.
		channelEntry.ForceClose = &dataformat.ForceClose{
			TXID:       hash.String(),
			Serialized: serialized,
			DelayBasePoint: &dataformat.BasePoint{
				Family: uint16(basepoint.Family),
				Index:  basepoint.Index,
				PubKey: hex.EncodeToString(
					basepoint.PubKey.SerializeCompressed(),
				),
			},
			RevocationBasePoint: &dataformat.BasePoint{
				PubKey: hex.EncodeToString(
					revpoint.PubKey.SerializeCompressed(),
				),
			},
			CommitPoint: hex.EncodeToString(
				point.SerializeCompressed(),
			),
			Outs: make(
				[]*dataformat.Out, len(localCommitTx.TxOut),
			),
			CSVDelay: channel.LocalChanCfg.CsvDelay,
		}
		for idx, out := range localCommitTx.TxOut {
			script, err := txscript.DisasmString(out.PkScript)
			if err != nil {
				return err
			}
			channelEntry.ForceClose.Outs[idx] = &dataformat.Out{
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
			log.Infof("Published TX %s, response: %s",
				hash.String(), response)
		}
	}

	summaryBytes, err := json.MarshalIndent(&dataformat.SummaryEntryFile{
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
