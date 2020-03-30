package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"time"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/btc"
	"github.com/guggero/chantools/dataformat"
	"github.com/guggero/chantools/lnd"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
)

type forceCloseCommand struct {
	RootKey   string `long:"rootkey" description:"BIP32 HD root key to use. Leave empty to prompt for lnd 24 word aezeed."`
	ChannelDB string `long:"channeldb" description:"The lnd channel.db file to use for force-closing channels."`
	Publish   bool   `long:"publish" description:"Should the force-closing TX be published to the chain API?"`
}

func (c *forceCloseCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	var (
		extendedKey *hdkeychain.ExtendedKey
		err         error
	)

	// Check that root key is valid or fall back to console input.
	switch {
	case c.RootKey != "":
		extendedKey, err = hdkeychain.NewKeyFromString(c.RootKey)

	default:
		extendedKey, _, err = rootKeyFromConsole()
	}
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	// Check that we have a channel DB.
	if c.ChannelDB == "" {
		return fmt.Errorf("rescue DB is required")
	}
	db, err := channeldb.Open(
		path.Dir(c.ChannelDB), channeldb.OptionSetSyncFreelist(true),
		channeldb.OptionReadOnly(true),
	)
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %v", err)
	}

	// Parse channel entries from any of the possible input files.
	entries, err := parseInputType(cfg)
	if err != nil {
		return err
	}
	return forceCloseChannels(extendedKey, entries, db, c.Publish)
}

func forceCloseChannels(extendedKey *hdkeychain.ExtendedKey,
	entries []*dataformat.SummaryEntry, chanDb *channeldb.DB,
	publish bool) error {

	channels, err := chanDb.FetchAllChannels()
	if err != nil {
		return err
	}
	api := &btc.ExplorerAPI{BaseURL: cfg.APIURL}
	signer := &lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}

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
		lc := &lnd.LightningChannel{
			LocalChanCfg:  channel.LocalChanCfg,
			RemoteChanCfg: channel.RemoteChanCfg,
			ChannelState:  channel,
			TXSigner:      signer,
		}
		err := lc.CreateSignDesc()
		if err != nil {
			return err
		}

		// Serialize transaction.
		signedTx, err := lc.SignedCommitTx()
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
			response, err := api.PublishTx(serialized)
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
