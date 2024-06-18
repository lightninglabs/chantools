package main

import (
	"errors"
	"fmt"

	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/spf13/cobra"
)

var (
	zombieBucket = []byte("zombie-index")
)

type dropGraphZombiesCommand struct {
	ChannelDB       string
	NodeIdentityKey string
	FixOnly         bool

	SingleChannel uint64

	cmd *cobra.Command
}

func newDropGraphZombiesCommand() *cobra.Command {
	cc := &dropGraphZombiesCommand{}
	cc.cmd = &cobra.Command{
		Use: "dropgraphzombies",
		Short: "Remove all channels identified as zombies from the " +
			"graph to force a re-sync of the graph",
		Long: `This command removes all channels that were identified as
zombies from the local graph.

This will cause lnd to re-download all those channels from the network and can
be helpful to fix a graph that is out of sync with the network.

CAUTION: Running this command will make it impossible to use the channel DB
with an older version of lnd. Downgrading is not possible and you'll need to
run lnd ` + lndVersion + ` or later after using this command!'`,
		Example: `chantools dropgraphzombies \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.ChannelDB, "channeldb", "", "lnd channel.db file to drop "+
			"zombies from",
	)

	return cc.cmd
}

func (c *dropGraphZombiesCommand) Execute(_ *cobra.Command, _ []string) error {
	// Check that we have a channel DB.
	if c.ChannelDB == "" {
		return errors.New("channel DB is required")
	}
	db, err := lnd.OpenDB(c.ChannelDB, false)
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %w", err)
	}
	defer func() { _ = db.Close() }()

	log.Infof("Dropping zombie channel bucket")

	rwTx, err := db.BeginReadWriteTx()
	if err != nil {
		return err
	}

	success := false
	defer func() {
		if !success {
			_ = rwTx.Rollback()
		}
	}()

	edges := rwTx.ReadWriteBucket(edgeBucket)
	if edges == nil {
		return channeldb.ErrGraphNoEdgesFound
	}

	if err := edges.DeleteNestedBucket(zombieBucket); err != nil {
		return err
	}

	success = true
	return rwTx.Commit()
}
