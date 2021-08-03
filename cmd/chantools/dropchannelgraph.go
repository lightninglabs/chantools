package main

import (
	"fmt"
	"github.com/guggero/chantools/lnd"
	"github.com/spf13/cobra"
)

var (
	nodeBucket      = []byte("graph-node")
	edgeBucket      = []byte("graph-edge")
	graphMetaBucket = []byte("graph-meta")
)

type dropChannelGraphCommand struct {
	ChannelDB string

	SingleChannel uint64

	cmd *cobra.Command
}

func newDropChannelGraphCommand() *cobra.Command {
	cc := &dropChannelGraphCommand{}
	cc.cmd = &cobra.Command{
		Use:   "dropchannelgraph",
		Short: "Remove all graph related data from a channel DB",
		Long: `This command removes all graph data from a channel DB,
forcing the lnd node to do a full graph sync.

Or if a single channel is specified, that channel is purged from the graph
without removing any other data.

CAUTION: Running this command will make it impossible to use the channel DB
with an older version of lnd. Downgrading is not possible and you'll need to
run lnd v0.13.1-beta or later after using this command!'`,
		Example: `chantools dropchannelgraph \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db

chantools dropchannelgraph \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db \
	--single_channel 726607861215512345`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.ChannelDB, "channeldb", "", "lnd channel.db file to dump "+
			"channels from",
	)
	cc.cmd.Flags().Uint64Var(
		&cc.SingleChannel, "single_channel", 0, "the single channel "+
			"identified by its short channel ID (CID) to remove "+
			"from the graph",
	)

	return cc.cmd
}

func (c *dropChannelGraphCommand) Execute(_ *cobra.Command, _ []string) error {
	// Check that we have a channel DB.
	if c.ChannelDB == "" {
		return fmt.Errorf("channel DB is required")
	}
	db, err := lnd.OpenDB(c.ChannelDB, false)
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %v", err)
	}
	defer func() { _ = db.Close() }()

	if c.SingleChannel != 0 {
		log.Infof("Removing single channel %d", c.SingleChannel)
		return db.ChannelGraph().DeleteChannelEdges(
			true, c.SingleChannel,
		)
	} else {
		log.Infof("Dropping all graph related buckets")

		rwTx, err := db.BeginReadWriteTx()
		if err != nil {
			return err
		}
		if err := rwTx.DeleteTopLevelBucket(nodeBucket); err != nil {
			return err
		}
		if err := rwTx.DeleteTopLevelBucket(edgeBucket); err != nil {
			return err
		}
		if err := rwTx.DeleteTopLevelBucket(graphMetaBucket); err != nil {
			return err
		}

		return rwTx.Commit()
	}
}
