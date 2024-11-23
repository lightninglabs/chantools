package main

import (
	"fmt"
	"path/filepath"

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
run lnd ` + lndVersion + ` or later after using this command!'`,
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
	var opts []lnd.DBOption

	// In case the channel DB is specified, we get the graph dir from it.
	if c.ChannelDB != "" {
		graphDir := filepath.Dir(c.ChannelDB)
		opts = append(opts, lnd.WithCustomGraphDir(graphDir))
	}

	dbConfig := GetDBConfig()

	db, err := lnd.OpenChannelDB(dbConfig, false, chainParams.Name, opts...)
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %w", err)
	}

	return db.Close()
}
