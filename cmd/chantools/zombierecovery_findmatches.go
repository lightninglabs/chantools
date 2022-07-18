package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/guggero/chantools/btc"
	"github.com/guggero/chantools/lnd"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/spf13/cobra"
)

var (
	patternRegistration = regexp.MustCompile(
		"(?m)(?s)ID: ([0-9a-f]{66})\nContact: (.*?)\n" +
			"Time: ")
)

type nodeInfo struct {
	PubKey       string   `json:"identity_pubkey"`
	Contact      string   `json:"contact"`
	PayoutAddr   string   `json:"payout_addr,omitempty"`
	MultisigKeys []string `json:"multisig_keys,omitempty"`
}

type channel struct {
	ChannelID     string `json:"short_channel_id"`
	ChanPoint     string `json:"chan_point"`
	Address       string `json:"address"`
	Capacity      int64  `json:"capacity"`
	txid          string
	vout          uint32
	ourKeyIndex   uint32
	ourKey        *btcec.PublicKey
	theirKey      *btcec.PublicKey
	witnessScript []byte
}

type match struct {
	Node1    *nodeInfo  `json:"node1"`
	Node2    *nodeInfo  `json:"node2"`
	Channels []*channel `json:"channels"`
}

type donePair struct {
	Node1 *nodeInfo `json:"node1"`
	Node2 *nodeInfo `json:"node2"`
	Msg   string    `json:"msg"`
}

func (p *donePair) matches(node1, node2 string) bool {
	return (p.Node1.PubKey == node1 && p.Node2.PubKey == node2) ||
		(p.Node1.PubKey == node2 && p.Node2.PubKey == node1)
}

type zombieRecoveryFindMatchesCommand struct {
	APIURL        string
	Registrations string
	ChannelGraph  string
	PairsDone     string

	cmd *cobra.Command
}

func newZombieRecoveryFindMatchesCommand() *cobra.Command {
	cc := &zombieRecoveryFindMatchesCommand{}
	cc.cmd = &cobra.Command{
		Use: "findmatches",
		Short: "[0/3] Match maker only: Find matches between " +
			"registered nodes",
		Long: `Match maker only: Runs through all the nodes that have
registered their ID on https://www.node-recovery.com and checks whether there
are any matches of channels between them by looking at the whole channel graph.

This command will be run by guggero and the result will be sent to the
registered nodes.`,
		Example: `chantools zombierecovery findmatches \
	--registrations data.txt \
	--channel_graph lncli_describegraph.json \
	--pairs_done pairs-done.json`,
		RunE: cc.Execute,
	}

	cc.cmd.Flags().StringVar(
		&cc.APIURL, "apiurl", defaultAPIURL, "API URL to use (must "+
			"be esplora compatible)",
	)
	cc.cmd.Flags().StringVar(
		&cc.Registrations, "registrations", "", "the raw data.txt "+
			"where the registrations are stored in",
	)
	cc.cmd.Flags().StringVar(
		&cc.ChannelGraph, "channel_graph", "", "the full LN channel "+
			"graph in the JSON format that the "+
			"'lncli describegraph' returns",
	)
	cc.cmd.Flags().StringVar(
		&cc.PairsDone, "pairs_done", "", "an optional file containing "+
			"all pairs that have already been contacted and "+
			"shouldn't be matched again",
	)

	return cc.cmd
}

func (c *zombieRecoveryFindMatchesCommand) Execute(_ *cobra.Command,
	_ []string) error {

	api := &btc.ExplorerAPI{BaseURL: c.APIURL}

	logFileBytes, err := ioutil.ReadFile(c.Registrations)
	if err != nil {
		return fmt.Errorf("error reading registrations file %s: %w",
			c.Registrations, err)
	}

	allMatches := patternRegistration.FindAllStringSubmatch(
		string(logFileBytes), -1,
	)
	registrations := make(map[string]string, len(allMatches))
	for _, groups := range allMatches {
		if _, err := pubKeyFromHex(groups[1]); err != nil {
			return fmt.Errorf("error parsing node ID: %w", err)
		}

		registrations[groups[1]] = groups[2]

		log.Infof("%s: %s", groups[1], groups[2])
	}

	graphBytes, err := ioutil.ReadFile(c.ChannelGraph)
	if err != nil {
		return fmt.Errorf("error reading graph JSON file %s: "+
			"%v", c.ChannelGraph, err)
	}
	graph := &lnrpc.ChannelGraph{}
	err = jsonpb.UnmarshalString(string(graphBytes), graph)
	if err != nil {
		return fmt.Errorf("error parsing graph JSON: %w", err)
	}

	var donePairs []*donePair
	if c.PairsDone != "" {
		donePairsBytes, err := readInput(c.PairsDone)
		if err != nil {
			return fmt.Errorf("error reading pairs JSON %s: %w",
				c.PairsDone, err)
		}
		decoder := json.NewDecoder(bytes.NewReader(donePairsBytes))
		err = decoder.Decode(&donePairs)
		if err != nil {
			return fmt.Errorf("error parsing pairs JSON %s: %w",
				c.PairsDone, err)
		}
	}

	// Loop through all nodes now.
	matches := make(map[string]map[string]*match)
	for node1, contact1 := range registrations {
		matches[node1] = make(map[string]*match)
		for node2, contact2 := range registrations {
			if node1 == node2 {
				continue
			}

			// We've already looked at this pair.
			if matches[node2][node1] != nil {
				continue
			}

			edges := lnd.FindCommonEdges(graph, node1, node2)
			if len(edges) > 0 {
				matches[node1][node2] = &match{
					Node1: &nodeInfo{
						PubKey:  node1,
						Contact: contact1,
					},
					Node2: &nodeInfo{
						PubKey:  node2,
						Contact: contact2,
					},
					Channels: make([]*channel, len(edges)),
				}

				for idx, edge := range edges {
					cid := fmt.Sprintf("%d", edge.ChannelId)
					c := &channel{
						ChannelID: cid,
						ChanPoint: edge.ChanPoint,
						Capacity:  edge.Capacity,
					}

					addr, err := api.Address(c.ChanPoint)
					if err == nil {
						c.Address = addr
					}

					matches[node1][node2].Channels[idx] = c
				}
			}
		}
	}

	// Write the matches to files.
	for node1, node1map := range matches {
		for node2, match := range node1map {
			if match == nil || isPairDone(donePairs, node1, node2) {
				continue
			}

			matchBytes, err := json.MarshalIndent(match, "", " ")
			if err != nil {
				return err
			}

			fileName := fmt.Sprintf("results/match-%s-%s-%s.json",
				time.Now().Format("2006-01-02"),
				node1, node2)
			log.Infof("Writing result to %s", fileName)
			err = ioutil.WriteFile(fileName, matchBytes, 0644)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func isPairDone(donePairs []*donePair, node1, node2 string) bool {
	for _, donePair := range donePairs {
		if donePair.matches(node1, node2) {
			return true
		}
	}

	return false
}
