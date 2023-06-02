package main

import (
	"fmt"

	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/spf13/cobra"
)

type chanBackupCommand struct {
	ChannelDB string
	MultiFile string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newChanBackupCommand() *cobra.Command {
	cc := &chanBackupCommand{}
	cc.cmd = &cobra.Command{
		Use:   "chanbackup",
		Short: "Create a channel.backup file from a channel database",
		Long: `This command creates a new channel.backup from a 
channel.db file.`,
		Example: `chantools chanbackup \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db \
	--multi_file new_channel_backup.backup`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.ChannelDB, "channeldb", "", "lnd channel.db file to "+
			"create the backup from",
	)
	cc.cmd.Flags().StringVar(
		&cc.MultiFile, "multi_file", "", "lnd channel.backup file to "+
			"create",
	)

	cc.rootKey = newRootKey(cc.cmd, "creating the backup")

	return cc.cmd
}

func (c *chanBackupCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	// Check that we have a backup file.
	if c.MultiFile == "" {
		return fmt.Errorf("backup file is required")
	}

	// Check that we have a channel DB.
	if c.ChannelDB == "" {
		return fmt.Errorf("channel DB is required")
	}
	db, err := lnd.OpenDB(c.ChannelDB, true)
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %w", err)
	}
	multiFile := chanbackup.NewMultiFile(c.MultiFile)
	keyRing := &lnd.HDKeyRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	return lnd.CreateChannelBackup(db, multiFile, keyRing)
}
