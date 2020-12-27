package main

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/guggero/chantools/dump"
	"github.com/guggero/chantools/lnd"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/spf13/cobra"
)

type dumpBackupCommand struct {
	MultiFile string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newDumpBackupCommand() *cobra.Command {
	cc := &dumpBackupCommand{}
	cc.cmd = &cobra.Command{
		Use:   "dumpbackup",
		Short: "Dump the content of a channel.backup file",
		Long: `This command dumps all information that is inside a 
channel.backup file in a human readable format.`,
		Example: `chantools dumpbackup --rootkey xprvxxxxxxxxxx \
	--multi_file ~/.lnd/data/chain/bitcoin/mainnet/channel.backup`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.MultiFile, "multi_file", "", "lnd channel.backup file to "+
			"dump",
	)

	cc.rootKey = newRootKey(cc.cmd, "decrypting the backup")

	return cc.cmd
}

func (c *dumpBackupCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	// Check that we have a backup file.
	if c.MultiFile == "" {
		return fmt.Errorf("backup file is required")
	}
	multiFile := chanbackup.NewMultiFile(c.MultiFile)
	keyRing := &lnd.HDKeyRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	return dumpChannelBackup(multiFile, keyRing)
}

func dumpChannelBackup(multiFile *chanbackup.MultiFile,
	ring keychain.KeyRing) error {

	multi, err := multiFile.ExtractMulti(ring)
	if err != nil {
		return fmt.Errorf("could not extract multi file: %v", err)
	}
	spew.Dump(dump.BackupMulti{
		Version:       multi.Version,
		StaticBackups: dump.BackupDump(multi, chainParams),
	})
	return nil
}
