package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/guggero/chantools/btc"
	"github.com/guggero/chantools/dataformat"
)

type summaryCommand struct{}

func (c *summaryCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	// Parse channel entries from any of the possible input files.
	entries, err := parseInputType(cfg)
	if err != nil {
		return err
	}
	return summarizeChannels(cfg.APIURL, entries)
}

func summarizeChannels(apiURL string,
	channels []*dataformat.SummaryEntry) error {

	summaryFile, err := btc.SummarizeChannels(apiURL, channels, log)
	if err != nil {
		return fmt.Errorf("error running summary: %v", err)
	}

	log.Info("Finished scanning.")
	log.Infof("Open channels: %d", summaryFile.OpenChannels)
	log.Infof("Sats in open channels: %d", summaryFile.FundsOpenChannels)
	log.Infof("Closed channels: %d", summaryFile.ClosedChannels)
	log.Infof(" --> force closed channels: %d",
		summaryFile.ForceClosedChannels)
	log.Infof(" --> coop closed channels: %d",
		summaryFile.CoopClosedChannels)
	log.Infof(" --> closed channels with all outputs spent: %d",
		summaryFile.FullySpentChannels)
	log.Infof(" --> closed channels with unspent outputs: %d",
		summaryFile.ChannelsWithUnspent)
	log.Infof(" --> closed channels with potentially our outputs: %d",
		summaryFile.ChannelsWithPotential)
	log.Infof("Sats in closed channels: %d", summaryFile.FundsClosedChannels)
	log.Infof(" --> closed channel sats that have been swept/spent: %d",
		summaryFile.FundsClosedSpent)
	log.Infof(" --> closed channel sats that are in force-close outputs: %d",
		summaryFile.FundsForceClose)
	log.Infof(" --> closed channel sats that are in coop close outputs: %d",
		summaryFile.FundsCoopClose)

	summaryBytes, err := json.MarshalIndent(summaryFile, "", " ")
	if err != nil {
		return err
	}
	fileName := fmt.Sprintf("results/summary-%s.json",
		time.Now().Format("2006-01-02-15-04-05"))
	log.Infof("Writing result to %s", fileName)
	return ioutil.WriteFile(fileName, summaryBytes, 0644)
}
