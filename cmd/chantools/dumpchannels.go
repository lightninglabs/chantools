package main

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
	"github.com/lightninglabs/chantools/dump"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/spf13/cobra"
)

type dumpChannelsCommand struct {
	ChannelDB    string
	Closed       bool
	Pending      bool
	WaitingClose bool

	cmd      *cobra.Command
	dbConfig *lnd.DB
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
	cc.cmd.Flags().BoolVar(
		&cc.Pending, "pending", false, "dump pending channels instead "+
			"of open",
	)
	cc.cmd.Flags().BoolVar(
		&cc.WaitingClose, "waiting_close", false, "dump waiting close "+
			"channels instead of open",
	)

	return cc.cmd
}

func (c *dumpChannelsCommand) Execute(_ *cobra.Command, _ []string) error {
	var opts []lnd.DBOption

	// In case the channel DB is specified, we get the graph dir from it.
	if c.ChannelDB != "" {
		graphDir := filepath.Dir(c.ChannelDB)
		opts = append(opts, lnd.WithCustomGraphDir(graphDir))
	}
	var dbConfig lnd.DB
	if c.dbConfig == nil {
		dbConfig = GetDBConfig()
	} else {
		dbConfig = *c.dbConfig
	}

	db, err := lnd.OpenChannelDB(dbConfig, true, chainParams.Name, opts...)
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %w", err)
	}
	defer func() { _ = db.Close() }()

	if (c.Closed && c.Pending) || (c.Closed && c.WaitingClose) ||
		(c.Pending && c.WaitingClose) ||
		(c.Closed && c.Pending && c.WaitingClose) {

		return errors.New("can only specify one flag at a time")
	}

	if c.Closed {
		return dumpClosedChannelInfo(db.ChannelStateDB())
	}
	if c.Pending {
		return dumpPendingChannelInfo(db.ChannelStateDB())
	}
	if c.WaitingClose {
		return dumpWaitingCloseChannelInfo(db.ChannelStateDB())
	}

	return dumpOpenChannelInfo(db.ChannelStateDB())
}

func dumpOpenChannelInfo(chanDb *channeldb.ChannelStateDB) error {
	channels, err := chanDb.FetchAllChannels()
	if err != nil {
		return err
	}

	dumpChannels, err := dump.OpenChannelDump(channels, chainParams)
	if err != nil {
		return fmt.Errorf("error converting to dump format: %w", err)
	}

	spew.Dump(dumpChannels)

	// For the tests, also log as trace level which is disabled by default.
	log.Tracef(spew.Sdump(dumpChannels))

	return nil
}

func dumpClosedChannelInfo(chanDb *channeldb.ChannelStateDB) error {
	channels, err := chanDb.FetchClosedChannels(false)
	if err != nil {
		return err
	}

	historicalChannels := make([]*channeldb.OpenChannel, len(channels))
	for idx := range channels {
		closedChan := channels[idx]
		histChan, err := chanDb.FetchHistoricalChannel(
			&closedChan.ChanPoint,
		)

		switch {
		// The channel was closed in a pre-historic version of lnd.
		// Ignore the error.
		case errors.Is(err, channeldb.ErrNoHistoricalBucket):
		case errors.Is(err, channeldb.ErrChannelNotFound):

		case err == nil:
			historicalChannels[idx] = histChan

		// Non-nil error not due to older versions of lnd.
		default:
			return err
		}
	}

	dumpChannels, err := dump.ClosedChannelDump(
		channels, historicalChannels, chainParams,
	)
	if err != nil {
		return fmt.Errorf("error converting to dump format: %w", err)
	}

	spew.Dump(dumpChannels)

	// For the tests, also log as trace level which is disabled by default.
	log.Tracef(spew.Sdump(dumpChannels))

	return nil
}

func dumpPendingChannelInfo(chanDb *channeldb.ChannelStateDB) error {
	channels, err := chanDb.FetchPendingChannels()
	if err != nil {
		return err
	}

	dumpChannels, err := dump.OpenChannelDump(channels, chainParams)
	if err != nil {
		return fmt.Errorf("error converting to dump format: %w", err)
	}

	spew.Dump(dumpChannels)

	// For the tests, also log as trace level which is disabled by default.
	log.Tracef(spew.Sdump(dumpChannels))

	return nil
}

func dumpWaitingCloseChannelInfo(chanDb *channeldb.ChannelStateDB) error {
	channels, err := chanDb.FetchWaitingCloseChannels()
	if err != nil {
		return err
	}

	dumpChannels, err := dump.OpenChannelDump(channels, chainParams)
	if err != nil {
		return fmt.Errorf("error converting to dump format: %w", err)
	}

	spew.Dump(dumpChannels)

	// For the tests, also log as trace level which is disabled by default.
	log.Tracef(spew.Sdump(dumpChannels))

	return nil
}
