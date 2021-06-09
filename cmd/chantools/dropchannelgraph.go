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

	cmd *cobra.Command
}

func newDropChannelGraphCommand() *cobra.Command {
	cc := &dropChannelGraphCommand{}
	cc.cmd = &cobra.Command{
		Use:   "dropchannelgraph",
		Short: "Remove all graph related data from a channel DB",
		Long: `This command removes all graph data from a channel DB,
forcing the lnd node to do a full graph sync.

CAUTION: Running this command will make it impossible to use the channel DB
with an older version of lnd. Downgrading is not possible and you'll need to
run lnd v0.12.1-beta or later after using this command!'`,
		Example: `chantools dropchannelgraph \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.ChannelDB, "channeldb", "", "lnd channel.db file to dump "+
			"channels from",
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
