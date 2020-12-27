package main

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/guggero/chantools/dump"
	"github.com/guggero/chantools/lnd"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/spf13/cobra"
)

type dumpChannelsCommand struct {
	ChannelDB string
	Closed    bool

	cmd *cobra.Command
}

func newDumpChannelsCommand() *cobra.Command {
	cc := &dumpChannelsCommand{}
	cc.cmd = &cobra.Command{
		Use: "dumpchannels",
		Short: "Dump all channel information from an lnd channel " +
			"database",
		Long: `This command dumps all open and pending channels from the
given lnd channel.db gile in a human readable format.`,
		Example: `chantools dumpchannels \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.ChannelDB, "channeldb", "", "lnd channel.db file to dump "+
			"channels from",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Closed, "closed", false, "dump closed channels instead of "+
			"open",
	)

	return cc.cmd
}

func (c *dumpChannelsCommand) Execute(_ *cobra.Command, _ []string) error {
	// Check that we have a channel DB.
	if c.ChannelDB == "" {
		return fmt.Errorf("channel DB is required")
	}
	db, err := lnd.OpenDB(c.ChannelDB, true)
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %v", err)
	}

	if c.Closed {
		return dumpClosedChannelInfo(db)
	}
	return dumpOpenChannelInfo(db)
}

func dumpOpenChannelInfo(chanDb *channeldb.DB) error {
	channels, err := chanDb.FetchAllChannels()
	if err != nil {
		return err
	}

	dumpChannels, err := dump.OpenChannelDump(channels, chainParams)
	if err != nil {
		return fmt.Errorf("error converting to dump format: %v", err)
	}

	spew.Dump(dumpChannels)
	return nil
}

func dumpClosedChannelInfo(chanDb *channeldb.DB) error {
	channels, err := chanDb.FetchClosedChannels(false)
	if err != nil {
		return err
	}

	dumpChannels, err := dump.ClosedChannelDump(channels, chainParams)
	if err != nil {
		return fmt.Errorf("error converting to dump format: %v", err)
	}

	spew.Dump(dumpChannels)
	return nil
}
