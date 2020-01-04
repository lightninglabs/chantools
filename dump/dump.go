package dump

import (
	"encoding/hex"
	"fmt"
	"net"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwire"
)

const (
	lndInternalDerivationPath = "m/1017'/0'/%d'/0/%d"
)

// BackupSingle is the information we want to dump from an lnd channel backup
// multi file. See `chanbackup.Multi` for information about the fields.
type BackupMulti struct {
	Version       chanbackup.MultiBackupVersion
	StaticBackups []BackupSingle
}

// BackupSingle is the information we want to dump from an lnd channel backup.
// See `chanbackup.Single` for information about the fields.
type BackupSingle struct {
	Version          chanbackup.SingleBackupVersion
	IsInitiator      bool
	ChainHash        string
	FundingOutpoint  string
	ShortChannelID   lnwire.ShortChannelID
	RemoteNodePub    string
	Addresses        []net.Addr
	Capacity         btcutil.Amount
	LocalChanCfg     ChannelConfig
	RemoteChanCfg    ChannelConfig
	ShaChainRootDesc KeyDescriptor
}

// OpenChannel is the information we want to dump from an open channel in lnd's
// channel DB. See `channeldb.OpenChannel` for information about the fields.
type OpenChannel struct {
	ChanType                channeldb.ChannelType
	ChainHash               chainhash.Hash
	FundingOutpoint         string
	ShortChannelID          lnwire.ShortChannelID
	IsPending               bool
	IsInitiator             bool
	ChanStatus              channeldb.ChannelStatus
	FundingBroadcastHeight  uint32
	NumConfsRequired        uint16
	ChannelFlags            lnwire.FundingFlag
	IdentityPub             string
	Capacity                btcutil.Amount
	TotalMSatSent           lnwire.MilliSatoshi
	TotalMSatReceived       lnwire.MilliSatoshi
	PerCommitPoint          string
	LocalChanCfg            ChannelConfig
	RemoteChanCfg           ChannelConfig
	LocalCommitment         channeldb.ChannelCommitment
	RemoteCommitment        channeldb.ChannelCommitment
	RemoteCurrentRevocation string
	RemoteNextRevocation    string
	FundingTxn              string
	LocalShutdownScript     lnwire.DeliveryAddress
	RemoteShutdownScript    lnwire.DeliveryAddress
}

// ChannelConfig is the information we want to dump from a channel
// configuration. See `channeldb.ChannelConfig` for more information about the
// fields.
type ChannelConfig struct {
	channeldb.ChannelConstraints
	MultiSigKey         KeyDescriptor
	RevocationBasePoint KeyDescriptor
	PaymentBasePoint    KeyDescriptor
	DelayBasePoint      KeyDescriptor
	HtlcBasePoint       KeyDescriptor
}

// KeyDescriptor is the information we want to dump from a key descriptor. See
// `keychain.KeyDescriptor` for more information about the fields.
type KeyDescriptor struct {
	Path   string
	PubKey string
}

func ToChannelConfig(cfg channeldb.ChannelConfig) ChannelConfig {
	return ChannelConfig{
		ChannelConstraints:  cfg.ChannelConstraints,
		MultiSigKey:         ToKeyDescriptor(cfg.MultiSigKey),
		RevocationBasePoint: ToKeyDescriptor(cfg.RevocationBasePoint),
		PaymentBasePoint:    ToKeyDescriptor(cfg.PaymentBasePoint),
		DelayBasePoint:      ToKeyDescriptor(cfg.DelayBasePoint),
		HtlcBasePoint:       ToKeyDescriptor(cfg.HtlcBasePoint),
	}
}

func ToKeyDescriptor(desc keychain.KeyDescriptor) KeyDescriptor {
	return KeyDescriptor{
		Path: fmt.Sprintf(
			lndInternalDerivationPath, desc.Family, desc.Index,
		),
		PubKey: PubKeyToString(desc.PubKey),
	}
}

func PubKeyToString(pubkey *btcec.PublicKey) string {
	if pubkey == nil {
		return "<nil>"
	}
	return hex.EncodeToString(pubkey.SerializeCompressed())
}
