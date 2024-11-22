package main

import (
	"fmt"
	"path/filepath"

	"github.com/lightninglabs/chantools/lnd"
	"github.com/spf13/cobra"
)

type deletePaymentsCommand struct {
	ChannelDB  string
	FailedOnly bool

	cmd *cobra.Command
}

func newDeletePaymentsCommand() *cobra.Command {
	cc := &deletePaymentsCommand{}
	cc.cmd = &cobra.Command{
		Use:   "deletepayments",
		Short: "Remove all (failed) payments from a channel DB",
		Long: `This command removes all payments from a channel DB.
If only the failed payments should be deleted (and not the successful ones), the
--failedonly flag can be specified.

CAUTION: Running this command will make it impossible to use the channel DB
with an older version of lnd. Downgrading is not possible and you'll need to
run lnd ` + lndVersion + ` or later after using this command!'`,
		Example: `chantools deletepayments --failedonly \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.ChannelDB, "channeldb", "", "lnd channel.db file to dump "+
			"channels from",
	)
	cc.cmd.Flags().BoolVar(
		&cc.FailedOnly, "failedonly", false, "don't delete all "+
			"payments, only failed ones",
	)

	return cc.cmd
}

func (c *deletePaymentsCommand) Execute(_ *cobra.Command, _ []string) error {
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
	defer func() { _ = db.Close() }()

	return db.DeletePayments(c.FailedOnly, false)
}
