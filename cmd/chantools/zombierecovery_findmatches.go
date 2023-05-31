package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/hasura/go-graphql-client"
	"github.com/lightninglabs/chantools/btc"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var (
	patternRegistration = regexp.MustCompile(
		"(?m)(?s)ID: ([0-9a-f]{66})\nContact: (.*?)\nTime: ",
	)

	defaultAmbossQueryDelay = 4 * time.Second

	initialTemplate = `SEND TO: {{.Contact}}

Hi

This is Oliver from node-recovery.com.
You recently registered your node ({{.Node1}}) with my website.

I have some good news! I found 
{{- if eq .NumChannels 1}} a match for a channel{{end}}
{{- if gt .NumChannels 1}} matches for {{.NumChannels}} channels{{end}}.
Attached you find the JSON files that contain all the info I have about your
node and the remote node (open with a text editor).

With those files you can close the channels and get your funds back. But you
need the cooperation of the remote peer. But because they also registered to the
same website, they should be aware of that and be willing to cooperate.

Please contact the remote peer with the contact information listed below (this
is what they registered with, I don't have additional contact information):

{{range $i, $peer := .Peers}}
Peer: {{$peer.PubKey}}
Contact: {{$peer.Contact}}

{{end}}
The document that describes what to do exactly is located here:
https://github.com/lightninglabs/chantools/blob/master/doc/zombierecovery.md

Good luck!

Oliver (guggero)


P.S.: If you don't want to be notified about future matches, please let me know.
`
)

type gqChannel struct {
	ChanPoint   string `graphql:"chan_point"`
	Capacity    string `graphql:"capacity"`
	ClosureInfo struct {
		ClosedHeight uint32 `graphql:"closed_height"`
	} `graphql:"closure_info"`
	Node1     string `graphql:"node1_pub"`
	Node2     string `graphql:"node2_pub"`
	ChannelID string `graphql:"long_channel_id"`
}

type gqGraphInfo struct {
	Channels struct {
		ChannelList struct {
			List []*gqChannel `graphql:"list"`
		} `graphql:"channel_list(page:{limit:$limit,offset:$offset})"`
	} `graphql:"channels"`
}

type gqGetNodeQuery struct {
	GetNode struct {
		GraphInfo *gqGraphInfo `graphql:"graph_info"`
	} `graphql:"getNode(pubkey: $pubkey)"`
}

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

type zombieRecoveryFindMatchesCommand struct {
	APIURL        string
	Registrations string
	AmbossKey     string
	AmbossDelay   time.Duration

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
	--ambosskey <API key>`,
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
		&cc.AmbossKey, "ambosskey", "", "the API key for the Amboss "+
			"GraphQL API",
	)
	cc.cmd.Flags().DurationVar(
		&cc.AmbossDelay, "ambossdelay", defaultAmbossQueryDelay,
		"the delay between each query to the Amboss GraphQL API",
	)

	return cc.cmd
}

func (c *zombieRecoveryFindMatchesCommand) Execute(_ *cobra.Command,
	_ []string) error {

	logFileBytes, err := os.ReadFile(c.Registrations)
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

		if registrations[groups[1]] != "" {
			registrations[groups[1]] += ", "
		}

		registrations[groups[1]] += groups[2]

		log.Infof("%s: %s", groups[1], groups[2])
	}

	api := &btc.ExplorerAPI{BaseURL: c.APIURL}
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: c.AmbossKey})
	httpClient := oauth2.NewClient(context.Background(), src)
	client := graphql.NewClient(
		"https://api.amboss.space/graphql", httpClient,
	)

	// Loop through all nodes now.
	matches := make(map[string]map[string]*match)
	idx := 0
	for node1, contact1 := range registrations {
		matches[node1] = make(map[string]*match)

		time.Sleep(c.AmbossDelay)
		log.Debugf("Fetching channels for node %d of %d", idx,
			len(registrations))
		idx++

		channels, err := fetchChannels(client, node1)
		if err != nil {
			return fmt.Errorf("error fetching channels for %s: %w",
				node1, err)
		}
		for _, node1Chan := range channels {
			peer := identifyPeer(node1Chan, node1)

			for node2, contact2 := range registrations {
				if node1 == node2 || node2 != peer {
					continue
				}

				if matches[node2][node1] != nil {
					continue
				}

				log.Debugf("Node 1 (%s, %s) has channel with "+
					"match (%s): %v", node1, contact1, peer,
					node1Chan.ChannelID)

				// This is a new match.
				if matches[node1][node2] == nil {
					matches[node1][node2] = &match{
						Node1: &nodeInfo{
							PubKey:  node1,
							Contact: contact1,
						},
						Node2: &nodeInfo{
							PubKey:  node2,
							Contact: contact2,
						},
					}
				}

				// Find the address of the channel.
				addr, err := api.Address(node1Chan.ChanPoint)
				if err != nil {
					return fmt.Errorf("error fetching "+
						"address for channel %s: %w",
						node1Chan.ChannelID, err)
				}
				capacity, err := strconv.ParseUint(
					node1Chan.Capacity, 10, 64,
				)
				if err != nil {
					return fmt.Errorf("error parsing "+
						"capacity for channel %s: %w",
						node1Chan.ChannelID, err)
				}

				// We've found a new match for this peer.
				newChan := &channel{
					ChannelID: node1Chan.ChannelID,
					ChanPoint: node1Chan.ChanPoint,
					Address:   addr,
					Capacity:  int64(capacity),
				}
				matches[node1][node2].Channels = append(
					matches[node1][node2].Channels,
					newChan,
				)
			}
		}
	}

	// To achieve a stable order, we sort the matches lexicographically by
	// their node key.
	node1IDs := make([]string, 0, len(matches))
	for node1 := range matches {
		node1IDs = append(node1IDs, node1)
	}
	sort.Strings(node1IDs)

	// Write the matches to files.
	for _, node1 := range node1IDs {
		node1map := matches[node1]

		tpl, err := template.New("initial").Parse(initialTemplate)
		if err != nil {
			return fmt.Errorf("error parsing template: %w", err)
		}

		tplVars := struct {
			Contact     string
			Node1       string
			NumChannels int
			Peers       []*nodeInfo
		}{
			Contact: registrations[node1],
			Node1:   node1,
		}

		folder := fmt.Sprintf("results/match-%s", node1)
		today := time.Now().Format("2006-01-02")
		for node2, match := range node1map {
			err = os.MkdirAll(folder, 0755)
			if err != nil {
				return err
			}

			matchBytes, err := json.MarshalIndent(match, "", " ")
			if err != nil {
				return err
			}

			fileName := fmt.Sprintf("%s/%s-%s.json",
				folder, node2, today)
			log.Infof("Writing result to %s", fileName)
			err = os.WriteFile(fileName, matchBytes, 0644)
			if err != nil {
				return err
			}

			tplVars.NumChannels += len(match.Channels)
			tplVars.Peers = append(tplVars.Peers, match.Node2)
		}

		if tplVars.NumChannels == 0 {
			continue
		}

		textFileName := fmt.Sprintf("%s/message-%s.txt", folder, today)
		file, err := os.OpenFile(
			textFileName, os.O_RDWR|os.O_CREATE, 0644,
		)
		if err != nil {
			return fmt.Errorf("error opening file %s: %w",
				textFileName, err)
		}

		err = tpl.Execute(file, tplVars)
		if err != nil {
			return fmt.Errorf("error executing template: %w", err)
		}
	}

	return nil
}

func fetchChannels(client *graphql.Client, pubkey string) ([]*gqChannel,
	error) {

	offset := 0.0
	limit := 50.0
	variables := map[string]interface{}{
		"pubkey": pubkey,
		"limit":  50.0,
		"offset": offset,
	}

	var channels []*gqChannel
	for {
		var query gqGetNodeQuery
		err := client.Query(context.Background(), &query, variables)
		if err != nil {
			if strings.Contains(err.Error(), "Too many requests") {
				time.Sleep(1 * time.Second)
				continue
			}

			return nil, err
		}

		channelList := query.GetNode.GraphInfo.Channels.ChannelList
		channels = append(channels, channelList.List...)

		if len(channelList.List) < int(limit) {
			break
		}

		offset += 50.0
		variables["offset"] = offset
	}

	return channels, nil
}

func identifyPeer(channel *gqChannel, node1 string) string {
	if channel.Node1 == node1 {
		return channel.Node2
	}
	if channel.Node2 == node1 {
		return channel.Node1
	}

	panic("peer not found")
}
