package main

import (
	"fmt"
	"path"

	"github.com/davecgh/go-spew/spew"
	"github.com/guggero/chantools/dump"
	"github.com/lightningnetwork/lnd/channeldb"
)

type dumpChannelsCommand struct {
	ChannelDB string `long:"channeldb" description:"The lnd channel.db file to dump the channels from."`
	Closed    bool   `long:"closed" description:"Dump all closed channels instead of all open channels."`
}

func (c *dumpChannelsCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	// Check that we have a channel DB.
	if c.ChannelDB == "" {
		return fmt.Errorf("channel DB is required")
	}
	db, err := channeldb.Open(
		path.Dir(c.ChannelDB), path.Base(c.ChannelDB),
		channeldb.OptionSetSyncFreelist(true),
		channeldb.OptionReadOnly(true),
	)
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