package chantools

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/guggero/chantools/dump"
	"github.com/lightningnetwork/lnd/chanbackup"
)

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
