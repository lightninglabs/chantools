package main

import (
	"fmt"
	"os"
	"time"

	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/btc"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/keychain"
)

type fixOldBackupCommand struct {
	RootKey   string `long:"rootkey" description:"BIP32 HD root key of the wallet that was used to create the backup. Leave empty to prompt for lnd 24 word aezeed."`
	MultiFile string `long:"multi_file" description:"The lnd channel.backup file to fix."`
}

func (c *fixOldBackupCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	var (
		extendedKey *hdkeychain.ExtendedKey
		err         error
	)

	// Check that root key is valid or fall back to console input.
	switch {
	case c.RootKey != "":
		extendedKey, err = hdkeychain.NewKeyFromString(c.RootKey)

	default:
		extendedKey, err = rootKeyFromConsole()
	}
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}
	// Check that we have a backup file.
	if c.MultiFile == "" {
		return fmt.Errorf("backup file is required")
	}
	multiFile := chanbackup.NewMultiFile(c.MultiFile)
	keyRing := &btc.HDKeyRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	return fixOldChannelBackup(multiFile, keyRing)
}

func fixOldChannelBackup(multiFile *chanbackup.MultiFile,
	ring *btc.HDKeyRing) error {

	multi, err := multiFile.ExtractMulti(ring)
	if err != nil {
		return fmt.Errorf("could not extract multi file: %v", err)
	}

	log.Infof("Checking shachain root of %d channels, this might take a "+
		"while.", len(multi.StaticBackups))
	fixedChannels := 0
	for _, single := range multi.StaticBackups {
		err := ring.CheckDescriptor(single.ShaChainRootDesc)
		switch err {
		case nil:
			continue

		case keychain.ErrCannotDerivePrivKey:
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
			single.ShaChainRootDesc = baseKeyDesc
			fixedChannels++

		default:
			return fmt.Errorf("could not check shachain root "+
				"descriptor: %v", err)
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
