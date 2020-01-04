package main

import (
	"fmt"

	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/davecgh/go-spew/spew"
	"github.com/guggero/chantools/btc"
	"github.com/guggero/chantools/dump"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/keychain"
)

type dumpBackupCommand struct {
	RootKey   string `long:"rootkey" description:"BIP32 HD root key of the wallet that was used to create the backup. Leave empty to prompt for lnd 24 word aezeed."`
	MultiFile string `long:"multi_file" description:"The lnd channel.backup file to dump."`
}

func (c *dumpBackupCommand) Execute(_ []string) error {
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
	keyRing := &btc.ChannelBackupEncryptionRing{
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
	dumpSingles := make([]dump.BackupSingle, len(multi.StaticBackups))
	for idx, single := range multi.StaticBackups {
		dumpSingles[idx] = dump.BackupSingle{
			Version:         single.Version,
			IsInitiator:     single.IsInitiator,
			ChainHash:       single.ChainHash.String(),
			FundingOutpoint: single.FundingOutpoint.String(),
			ShortChannelID:  single.ShortChannelID,
			RemoteNodePub: dump.PubKeyToString(
				single.RemoteNodePub,
			),
			Addresses: single.Addresses,
			Capacity:  single.Capacity,
			LocalChanCfg: dump.ToChannelConfig(
				single.LocalChanCfg,
			),
			RemoteChanCfg: dump.ToChannelConfig(
				single.RemoteChanCfg,
			),
			ShaChainRootDesc: dump.ToKeyDescriptor(
				single.ShaChainRootDesc,
			),
		}
	}

	spew.Dump(dump.BackupMulti{
		Version:       multi.Version,
		StaticBackups: dumpSingles,
	})
	return nil
}
