package main

import (
	"fmt"
	"path"

	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/lnd"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/channeldb"
)

type chanBackupCommand struct {
	RootKey   string `long:"rootkey" description:"BIP32 HD root key of the wallet that should be used to create the backup. Leave empty to prompt for lnd 24 word aezeed."`
	ChannelDB string `long:"channeldb" description:"The lnd channel.db file to create the backup from."`
	MultiFile string `long:"multi_file" description:"The lnd channel.backup file to create."`
}

func (c *chanBackupCommand) Execute(_ []string) error {
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
		extendedKey, _, err = lnd.ReadAezeedFromTerminal(chainParams)
	}
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	// Check that we have a backup file.
	if c.MultiFile == "" {
		return fmt.Errorf("backup file is required")
	}

	// Check that we have a channel DB.
	if c.ChannelDB == "" {
		return fmt.Errorf("channel DB is required")
	}
	db, err := channeldb.Open(
		path.Dir(c.ChannelDB), path.Base(c.ChannelDB),
		channeldb.OptionSetSyncFreelist(true),
		channeldb.OptionReadOnly(true),
	)
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %v", err)
	}
	multiFile := chanbackup.NewMultiFile(c.MultiFile)
	keyRing := &lnd.HDKeyRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	return lnd.CreateChannelBackup(db, multiFile, keyRing)
}
