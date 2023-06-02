package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/spf13/cobra"
)

type filterBackupCommand struct {
	MultiFile string
	Discard   string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newFilterBackupCommand() *cobra.Command {
	cc := &filterBackupCommand{}
	cc.cmd = &cobra.Command{
		Use: "filterbackup",
		Short: "Filter an lnd channel.backup file and remove certain " +
			"channels",
		Long: `Filter an lnd channel.backup file by removing certain 
channels (identified by their funding transaction outpoints).`,
		Example: `chantools filterbackup \
	--multi_file ~/.lnd/data/chain/bitcoin/mainnet/channel.backup \
	--discard 2abcdef2b2bffaaa...db0abadd:1,4abcdef2b2bffaaa...db8abadd:0`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.MultiFile, "multi_file", "", "lnd channel.backup file to "+
			"filter",
	)
	cc.cmd.Flags().StringVar(
		&cc.Discard, "discard", "", "comma separated list of channel "+
			"funding outpoints (format <fundingTXID>:<index>) to "+
			"remove from the backup file",
	)

	cc.rootKey = newRootKey(cc.cmd, "decrypting the backup")

	return cc.cmd
}

func (c *filterBackupCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	// Parse discard filter.
	discard := strings.Split(c.Discard, ",")

	// Check that we have a backup file.
	if c.MultiFile == "" {
		return fmt.Errorf("backup file is required")
	}
	multiFile := chanbackup.NewMultiFile(c.MultiFile)
	keyRing := &lnd.HDKeyRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	return filterChannelBackup(multiFile, keyRing, discard)
}

func filterChannelBackup(multiFile *chanbackup.MultiFile, ring keychain.KeyRing,
	discard []string) error {

	multi, err := multiFile.ExtractMulti(ring)
	if err != nil {
		return fmt.Errorf("could not extract multi file: %w", err)
	}

	keep := make([]chanbackup.Single, 0, len(multi.StaticBackups))
	for _, single := range multi.StaticBackups {
		found := false
		for _, discardChanPoint := range discard {
			if single.FundingOutpoint.String() == discardChanPoint {
				found = true
			}
		}
		if found {
			continue
		}
		keep = append(keep, single)
	}
	multi.StaticBackups = keep

	fileName := fmt.Sprintf("results/backup-filtered-%s.backup",
		time.Now().Format("2006-01-02-15-04-05"))
	log.Infof("Writing result to %s", fileName)
	f, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	err = multi.PackToWriter(f, ring)
	_ = f.Close()
	if err != nil {
		return err
	}
	return nil
}
