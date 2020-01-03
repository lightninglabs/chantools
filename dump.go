package chantools

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/keychain"
)

const (
	lndInternalDerivationPath = "m/1017'/0'/%d'/0/%d"
)

// dumpChanCfg is the information we want to dump from a channel configuration.
// See `channeldb.ChannelConfig` for more information about the fields.
type dumpChanCfg struct {
	channeldb.ChannelConstraints
	MultiSigKey         dumpDescriptor
	RevocationBasePoint dumpDescriptor
	PaymentBasePoint    dumpDescriptor
	DelayBasePoint      dumpDescriptor
	HtlcBasePoint       dumpDescriptor
}

type dumpDescriptor struct {
	Path   string
	Pubkey string
}

func toDumpChanCfg(cfg channeldb.ChannelConfig) dumpChanCfg {
	return dumpChanCfg{
		ChannelConstraints:  cfg.ChannelConstraints,
		MultiSigKey:         toDumpDescriptor(cfg.MultiSigKey),
		RevocationBasePoint: toDumpDescriptor(cfg.RevocationBasePoint),
		PaymentBasePoint:    toDumpDescriptor(cfg.PaymentBasePoint),
		DelayBasePoint:      toDumpDescriptor(cfg.DelayBasePoint),
		HtlcBasePoint:       toDumpDescriptor(cfg.HtlcBasePoint),
	}
}

func toDumpDescriptor(desc keychain.KeyDescriptor) dumpDescriptor {
	return dumpDescriptor{
		Path: fmt.Sprintf(
			lndInternalDerivationPath, desc.Family, desc.Index,
		),
		Pubkey: pubKeyToString(desc.PubKey),
	}
}

func pubKeyToString(pubkey *btcec.PublicKey) string {
	if pubkey == nil {
		return "<nil>"
	}
	return hex.EncodeToString(pubkey.SerializeCompressed())
}
