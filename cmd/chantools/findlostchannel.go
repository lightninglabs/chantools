package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/spf13/cobra"
)

type findLostChannelCommand struct {
	ChannelGraph string
	ChannelPoint string

	TorProxy string

	ConnectTimeout time.Duration

	rootKey *rootKey
	cmd     *cobra.Command
}

func newFindLostChannelCommand() *cobra.Command {
	cc := &findLostChannelCommand{}
	cc.cmd = &cobra.Command{
		Use:   "findlostchannel",
		Short: "Try to find out what node a channel was made with",
		Long: `Connects to _all_ nodes given in the graph file and 
attempts to find out if the node has knowledge of the given channel.
Can be used to try to recover funds if the peer is not known (because no 
channel.backup was created).

The graph.json file can be created with 'lncli describegraph > graph.json' on
any node in the network.`,
		Example: `chantools findlostchannel \
	--channel_graph graph.json \
	--channel_point abcdef01234...:x`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.ChannelGraph, "channel_graph", "", "the channel graph "+
			"file as a JSON file, containing all nodes and their "+
			"network addresses of the network",
	)
	cc.cmd.Flags().StringVar(
		&cc.ChannelPoint, "channel_point", "", "funding transaction "+
			"outpoint of the channel to attempt to find the peer "+
			"for (<txid>:<txindex>)",
	)
	cc.cmd.Flags().StringVar(
		&cc.TorProxy, "torproxy", "", "SOCKS5 proxy to use for Tor "+
			"connections (to .onion addresses)",
	)
	cc.cmd.Flags().DurationVar(
		&cc.ConnectTimeout, "connect_timeout", time.Second*30, "time "+
			"to wait for a connection to be established",
	)
	cc.rootKey = newRootKey(cc.cmd, "deriving the identity key")

	return cc.cmd
}

func (c *findLostChannelCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	identityPath := lnd.IdentityPath(chainParams)
	child, pubKey, _, err := lnd.DeriveKey(
		extendedKey, identityPath, chainParams,
	)
	if err != nil {
		return fmt.Errorf("could not derive identity key: %w", err)
	}
	identityPriv, err := child.ECPrivKey()
	if err != nil {
		return fmt.Errorf("could not get identity private key: %w", err)
	}
	identityECDH := &keychain.PrivKeyECDH{
		PrivKey: identityPriv,
	}

	outPoint, err := parseOutPoint(c.ChannelPoint)
	if err != nil {
		return fmt.Errorf("error parsing channel point: %w", err)
	}

	graphBytes, err := os.ReadFile(c.ChannelGraph)
	if err != nil {
		return fmt.Errorf("error reading graph JSON file %s: "+
			"%v", c.ChannelGraph, err)
	}
	graph := &lnrpc.ChannelGraph{}
	err = jsonpb.UnmarshalString(string(graphBytes), graph)
	if err != nil {
		return fmt.Errorf("error parsing graph JSON: %w", err)
	}

	for idx := range graph.Nodes {
		node := graph.Nodes[idx]
		if node.PubKey == "" {
			continue
		}

		for _, addr := range node.Addresses {
			nodeAddr := fmt.Sprintf("%s@%s", node.PubKey, addr.Addr)
			p, err := connectPeer(
				nodeAddr, c.TorProxy, pubKey, identityECDH,
				c.ConnectTimeout,
			)
			if err != nil {
				log.Infof("Error with node %s: %v", node.PubKey,
					err)
				continue
			}

			channelID := lnwire.NewChanIDFromOutPoint(outPoint)

			// Channel ID (32 byte) + u16 for the data length (which
			// will be 0).
			data := make([]byte, 34)
			copy(data[:32], channelID[:])

			log.Infof("Sending channel re-establish to peer to "+
				"trigger force close of channel %v", outPoint)

			err = p.SendMessageLazy(
				true, &lnwire.ChannelReestablish{
					ChanID: channelID,
				},
			)
			if err != nil {
				return err
			}

			// Continue with next node.
			break
		}
	}

	return nil
}
