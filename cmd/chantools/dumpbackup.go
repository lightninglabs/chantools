package main

import (
	"fmt"

	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/davecgh/go-spew/spew"
	"github.com/guggero/chantools/btc"
	"github.com/guggero/chantools/dump"
	"github.com/lightningnetwork/lnd/chanbackup"
)

type dumpBackupCommand struct {
	RootKey   string `long:"rootkey" description:"BIP32 HD root key of the wallet that was used to create the backup."`
	MultiFile string `long:"multi_file" description:"The lnd channel.backup file to dump."`
}

func (c *dumpBackupCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	// Check that root key is valid.
	if c.RootKey == "" {
		return fmt.Errorf("root key is required")
	}
	extendedKey, err := hdkeychain.NewKeyFromString(c.RootKey)
	if err != nil {
		return fmt.Errorf("error parsing root key: %v", err)
	}

	// Check that we have a backup file.
	if c.MultiFile == "" {
		return fmt.Errorf("backup file is required")
	}
	multiFile := chanbackup.NewMultiFile(c.MultiFile)
	multi, err := multiFile.ExtractMulti(&btc.ChannelBackupEncryptionRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	})
	if err != nil {
		return fmt.Errorf("could not extract multi file: %v", err)
	}
	return dumpChannelBackup(multi)
}

func dumpChannelBackup(multi *chanbackup.Multi) error {
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
