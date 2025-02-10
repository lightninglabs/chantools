package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/dataformat"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/spf13/cobra"
)

type summaryCommand struct {
	APIURL string

	Ancient      bool
	AncientStats string

	inputs *inputFlags
	cmd    *cobra.Command
}

func newSummaryCommand() *cobra.Command {
	cc := &summaryCommand{}
	cc.cmd = &cobra.Command{
		Use: "summary",
		Short: "Compile a summary about the current state of " +
			"channels",
		Long: `From a list of channels, find out what their state is by
querying the funding transaction on a block explorer API.`,
		Example: `lncli listchannels | chantools summary --listchannels -

chantools summary --fromchanneldb ~/.lnd/data/graph/mainnet/channel.db`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.APIURL, "apiurl", defaultAPIURL, "API URL to use (must "+
			"be esplora compatible)",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Ancient, "ancient", false, "Create summary of ancient "+
			"channel closes with un-swept outputs",
	)
	cc.cmd.Flags().StringVar(
		&cc.AncientStats, "ancientstats", "", "Create summary of "+
			"ancient channel closes with un-swept outputs and "+
			"print stats for the given list of channels",
	)

	cc.inputs = newInputFlags(cc.cmd)

	return cc.cmd
}

func (c *summaryCommand) Execute(_ *cobra.Command, _ []string) error {
	if c.AncientStats != "" {
		return summarizeAncientChannelOutputs(c.APIURL, c.AncientStats)
	}

	// Parse channel entries from any of the possible input files.
	entries, err := c.inputs.parseInputType()
	if err != nil {
		return err
	}

	if c.Ancient {
		return summarizeAncientChannels(c.APIURL, entries)
	}

	return summarizeChannels(c.APIURL, entries)
}

func summarizeChannels(apiURL string,
	channels []*dataformat.SummaryEntry) error {

	api := newExplorerAPI(apiURL)
	summaryFile, err := btc.SummarizeChannels(api, channels, log)
	if err != nil {
		return fmt.Errorf("error running summary: %w", err)
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
	return os.WriteFile(fileName, summaryBytes, 0644)
}

func summarizeAncientChannels(apiURL string,
	channels []*dataformat.SummaryEntry) error {

	api := newExplorerAPI(apiURL)

	var results []*ancientChannel
	for _, target := range channels {
		if target.ClosingTX == nil {
			continue
		}

		closeTx := target.ClosingTX
		if !closeTx.ForceClose {
			continue
		}

		if closeTx.AllOutsSpent {
			continue
		}

		if closeTx.OurAddr != "" {
			log.Infof("Channel %s has potential funds: %d in %s",
				target.ChannelPoint, target.LocalBalance,
				closeTx.OurAddr)
		}

		if target.LocalUnrevokedCommitPoint == "" {
			log.Warnf("Channel %s has no unrevoked commit point",
				target.ChannelPoint)
			continue
		}

		if closeTx.ToRemoteAddr == "" {
			log.Warnf("Close TX %s has no remote address",
				closeTx.TXID)
			continue
		}

		addr, err := lnd.ParseAddress(closeTx.ToRemoteAddr, chainParams)
		if err != nil {
			return fmt.Errorf("error parsing address %s of %s: %w",
				closeTx.ToRemoteAddr, closeTx.TXID, err)
		}

		if _, ok := addr.(*btcutil.AddressWitnessPubKeyHash); !ok {
			log.Infof("Channel close %s has non-p2wkh output: %s",
				closeTx.TXID, closeTx.ToRemoteAddr)
			continue
		}

		tx, err := api.Transaction(closeTx.TXID)
		if err != nil {
			return fmt.Errorf("error fetching transaction %s: %w",
				closeTx.TXID, err)
		}

		for idx, txOut := range tx.Vout {
			if txOut.Outspend.Spent {
				continue
			}

			if txOut.ScriptPubkeyAddr == closeTx.ToRemoteAddr {
				results = append(results, &ancientChannel{
					OP: fmt.Sprintf("%s:%d", closeTx.TXID,
						idx),
					Addr: closeTx.ToRemoteAddr,
					CP:   target.LocalUnrevokedCommitPoint,
				})
			}
		}
	}

	summaryBytes, err := json.MarshalIndent(results, "", " ")
	if err != nil {
		return err
	}
	fileName := fmt.Sprintf("results/summary-ancient-%s.json",
		time.Now().Format("2006-01-02-15-04-05"))
	log.Infof("Writing result to %s", fileName)
	return os.WriteFile(fileName, summaryBytes, 0644)
}

func summarizeAncientChannelOutputs(apiURL, ancientFile string) error {
	jsonBytes, err := os.ReadFile(ancientFile)
	if err != nil {
		return fmt.Errorf("error reading file %s: %w", ancientFile, err)
	}

	var ancients []ancientChannel
	err = json.Unmarshal(jsonBytes, &ancients)
	if err != nil {
		return fmt.Errorf("error unmarshalling ancient channels: %w",
			err)
	}

	var (
		api         = newExplorerAPI(apiURL)
		numUnspents uint32
		unspentSats uint64
	)
	for _, channel := range ancients {
		unspents, err := api.Unspent(channel.Addr)
		if err != nil {
			return fmt.Errorf("error fetching unspents for %s: %w",
				channel.Addr, err)
		}

		if len(unspents) > 1 {
			log.Infof("Address %s has multiple unspents",
				channel.Addr)
		}
		for _, unspent := range unspents {
			if unspent.Outspend.Spent {
				continue
			}

			numUnspents++
			unspentSats += unspent.Value
		}
	}

	log.Infof("Found %d unspent outputs with %d sats", numUnspents,
		unspentSats)

	return nil
}
