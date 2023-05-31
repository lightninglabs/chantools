package main

import (
	"fmt"

	"github.com/lightninglabs/chantools/lnd"
	"github.com/spf13/cobra"
)

type migrateDBCommand struct {
	ChannelDB string

	cmd *cobra.Command
}

func newMigrateDBCommand() *cobra.Command {
	cc := &migrateDBCommand{}
	cc.cmd = &cobra.Command{
		Use:   "migratedb",
		Short: "Apply all recent lnd channel database migrations",
		Long: `This command opens an lnd channel database in write mode
and applies all recent database migrations to it. This can be used to update
an old database file to be compatible with the current version that chantools
needs to read the database content.

CAUTION: Running this command will make it impossible to use the channel DB
with an older version of lnd. Downgrading is not possible and you'll need to
run lnd v0.16.0-beta or later after using this command!'`,
		Example: `chantools migratedb \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.ChannelDB, "channeldb", "", "lnd channel.db file to "+
			"migrate",
	)

	return cc.cmd
}

func (c *migrateDBCommand) Execute(_ *cobra.Command, _ []string) error {
	// Check that we have a channel DB.
	if c.ChannelDB == "" {
		return fmt.Errorf("channel DB is required")
	}
	db, err := lnd.OpenDB(c.ChannelDB, false)
	if err != nil {
		return fmt.Errorf("error opening DB: %w", err)
	}

	return db.Close()
}
