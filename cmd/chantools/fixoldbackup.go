package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/spf13/cobra"
)

type fixOldBackupCommand struct {
	MultiFile string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newFixOldBackupCommand() *cobra.Command {
	cc := &fixOldBackupCommand{}
	cc.cmd = &cobra.Command{
		Use: "fixoldbackup",
		Short: "Fixes an old channel.backup file that is affected by " +
			"the lnd issue #3881 (unable to derive shachain root " +
			"key)",
		Long: `Fixes an old channel.backup file that is affected by the
lnd issue [#3881](https://github.com/lightningnetwork/lnd/issues/3881)
(<code>[lncli] unable to restore chan backups: rpc error: code = Unknown desc =
unable to unpack chan backup: unable to derive shachain root key: unable to
derive private key</code>).`,
		Example: `chantools fixoldbackup \
	--multi_file ~/.lnd/data/chain/bitcoin/mainnet/channel.backup`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.MultiFile, "multi_file", "", "lnd channel.backup file to "+
			"fix",
	)

	cc.rootKey = newRootKey(cc.cmd, "decrypting the backup")

	return cc.cmd
}

func (c *fixOldBackupCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
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
	return fixOldChannelBackup(multiFile, keyRing)
}

func fixOldChannelBackup(multiFile *chanbackup.MultiFile,
	ring *lnd.HDKeyRing) error {

	multi, err := multiFile.ExtractMulti(ring)
	if err != nil {
		return fmt.Errorf("could not extract multi file: %w", err)
	}

	log.Infof("Checking shachain root of %d channels, this might take a "+
		"while.", len(multi.StaticBackups))
	fixedChannels := 0
	for idx, single := range multi.StaticBackups {
		err := ring.CheckDescriptor(single.ShaChainRootDesc)
		switch {
		case err == nil:
			continue

		case errors.Is(err, keychain.ErrCannotDerivePrivKey):
			// Fix the incorrect descriptor by deriving a default
			// one and overwriting it in the backup.
			log.Infof("The shachain root for channel %s could "+
				"not be derived, must be in old format. "+
				"Fixing...", single.FundingOutpoint.String())
			baseKeyDesc, err := ring.DeriveKey(keychain.KeyLocator{
				Family: keychain.KeyFamilyRevocationRoot,
				Index:  0,
			})
			if err != nil {
				return err
			}
			multi.StaticBackups[idx].ShaChainRootDesc = baseKeyDesc
			fixedChannels++

		default:
			return fmt.Errorf("could not check shachain root "+
				"descriptor: %w", err)
		}
	}
	if fixedChannels == 0 {
		log.Info("No channels were affected by issue #3881, nothing " +
			"to fix.")
		return nil
	}

	log.Infof("Fixed shachain root of %d channels.", fixedChannels)
	fileName := fmt.Sprintf("results/backup-fixed-%s.backup",
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
