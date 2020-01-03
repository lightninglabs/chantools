package chantools

import (
	"net"

	"github.com/btcsuite/btcutil"
	"github.com/davecgh/go-spew/spew"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/lnwire"
)

type dumpMulti struct {
	Version       chanbackup.MultiBackupVersion
	StaticBackups []dumpSingle
}

// dumpSingle is the information we want to dump from an lnd channel backup.
// See `chanbackup.Single` for information about the fields.
type dumpSingle struct {
	Version          chanbackup.SingleBackupVersion
	IsInitiator      bool
	ChainHash        string
	FundingOutpoint  string
	ShortChannelID   lnwire.ShortChannelID
	RemoteNodePub    string
	Addresses        []net.Addr
	Capacity         btcutil.Amount
	LocalChanCfg     dumpChanCfg
	RemoteChanCfg    dumpChanCfg
	ShaChainRootDesc dumpDescriptor
}

func dumpChannelBackup(multi *chanbackup.Multi) error {
	dumpSingles := make([]dumpSingle, len(multi.StaticBackups))
	for idx, single := range multi.StaticBackups {
		dumpSingles[idx] = dumpSingle{
			Version:          single.Version,
			IsInitiator:      single.IsInitiator,
			ChainHash:        single.ChainHash.String(),
			FundingOutpoint:  single.FundingOutpoint.String(),
			ShortChannelID:   single.ShortChannelID,
			RemoteNodePub:    pubKeyToString(single.RemoteNodePub),
			Addresses:        single.Addresses,
			Capacity:         single.Capacity,
			LocalChanCfg:     toDumpChanCfg(single.LocalChanCfg),
			RemoteChanCfg:    toDumpChanCfg(single.RemoteChanCfg),
			ShaChainRootDesc: toDumpDescriptor(
				single.ShaChainRootDesc,
			),
		}
	}

	spew.Dump(dumpMulti{
		Version:       multi.Version,
		StaticBackups: dumpSingles,
	})
	return nil
}
