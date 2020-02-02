package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/btc"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/keychain"
)

type filterBackupCommand struct {
	RootKey   string `long:"rootkey" description:"BIP32 HD root key of the wallet that was used to create the backup. Leave empty to prompt for lnd 24 word aezeed."`
	MultiFile string `long:"multi_file" description:"The lnd channel.backup file to filter."`
	Discard   string `long:"discard" description:"A comma separated list of channel funding outpoints (format <fundingTXID>:<index>) to remove from the backup file."`
}

func (c *filterBackupCommand) Execute(_ []string) error {
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
		extendedKey, _, err = rootKeyFromConsole()
	}
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	// Parse discard filter.
	discard := strings.Split(c.Discard, ",")

	// Check that we have a backup file.
	if c.MultiFile == "" {
		return fmt.Errorf("backup file is required")
	}
	multiFile := chanbackup.NewMultiFile(c.MultiFile)
	keyRing := &btc.HDKeyRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	return filterChannelBackup(multiFile, keyRing, discard)
}

func filterChannelBackup(multiFile *chanbackup.MultiFile, ring keychain.KeyRing,
	discard []string) error {

	multi, err := multiFile.ExtractMulti(ring)
	if err != nil {
		return fmt.Errorf("could not extract multi file: %v", err)
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
