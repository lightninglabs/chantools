package dump

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwire"
)

const (
	lndInternalDerivationPath = "m/1017'/%d'/%d'/0/%d"
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

// ClosedChannel is the information we want to dump from a closed channel in
// lnd's channel DB. See `channeldb.ChannelCloseSummary` for information about
// the fields.
type ClosedChannel struct {
	ChanPoint               string
	ShortChanID             lnwire.ShortChannelID
	ChainHash               chainhash.Hash
	ClosingTXID             string
	RemotePub               string
	Capacity                btcutil.Amount
	CloseHeight             uint32
	SettledBalance          btcutil.Amount
	TimeLockedBalance       btcutil.Amount
	CloseType               string
	IsPending               bool
	RemoteCurrentRevocation string
	RemoteNextRevocation    string
	LocalChanConfig         ChannelConfig
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

// OpenChannelDump converts the open channels in the given channel DB into a
// dumpable format.
func OpenChannelDump(channels []*channeldb.OpenChannel,
	params *chaincfg.Params) ([]OpenChannel, error) {

	dumpChannels := make([]OpenChannel, len(channels))
	for idx, channel := range channels {
		var buf bytes.Buffer
		if channel.FundingTxn != nil {
			err := channel.FundingTxn.Serialize(&buf)
			if err != nil {
				return nil, err
			}
		}
		revPreimage, err := channel.RevocationProducer.AtIndex(
			channel.LocalCommitment.CommitHeight,
		)
		if err != nil {
			return nil, err
		}
		perCommitPoint := input.ComputeCommitmentPoint(revPreimage[:])

		dumpChannels[idx] = OpenChannel{
			ChanType:               channel.ChanType,
			ChainHash:              channel.ChainHash,
			FundingOutpoint:        channel.FundingOutpoint.String(),
			ShortChannelID:         channel.ShortChannelID,
			IsPending:              channel.IsPending,
			IsInitiator:            channel.IsInitiator,
			ChanStatus:             channel.ChanStatus(),
			FundingBroadcastHeight: channel.FundingBroadcastHeight,
			NumConfsRequired:       channel.NumConfsRequired,
			ChannelFlags:           channel.ChannelFlags,
			IdentityPub: PubKeyToString(
				channel.IdentityPub,
			),
			Capacity:          channel.Capacity,
			TotalMSatSent:     channel.TotalMSatSent,
			TotalMSatReceived: channel.TotalMSatReceived,
			PerCommitPoint:    PubKeyToString(perCommitPoint),
			LocalChanCfg: ToChannelConfig(
				params, channel.LocalChanCfg,
			),
			RemoteChanCfg: ToChannelConfig(
				params, channel.RemoteChanCfg,
			),
			LocalCommitment:  channel.LocalCommitment,
			RemoteCommitment: channel.RemoteCommitment,
			RemoteCurrentRevocation: PubKeyToString(
				channel.RemoteCurrentRevocation,
			),
			RemoteNextRevocation: PubKeyToString(
				channel.RemoteNextRevocation,
			),
			FundingTxn:           hex.EncodeToString(buf.Bytes()),
			LocalShutdownScript:  channel.LocalShutdownScript,
			RemoteShutdownScript: channel.RemoteShutdownScript,
		}
	}
	return dumpChannels, nil
}

// ClosedChannelDump converts the closed channels in the given channel DB into a
// dumpable format.
func ClosedChannelDump(channels []*channeldb.ChannelCloseSummary,
	params *chaincfg.Params) ([]ClosedChannel, error) {

	dumpChannels := make([]ClosedChannel, len(channels))
	for idx, channel := range channels {
		dumpChannels[idx] = ClosedChannel{
			ChanPoint:         channel.ChanPoint.String(),
			ShortChanID:       channel.ShortChanID,
			ChainHash:         channel.ChainHash,
			ClosingTXID:       channel.ClosingTXID.String(),
			RemotePub:         PubKeyToString(channel.RemotePub),
			Capacity:          channel.Capacity,
			CloseHeight:       channel.CloseHeight,
			SettledBalance:    channel.SettledBalance,
			TimeLockedBalance: channel.TimeLockedBalance,
			CloseType: fmt.Sprintf(
				"%d", channel.CloseType,
			),
			IsPending: channel.IsPending,
			RemoteCurrentRevocation: PubKeyToString(
				channel.RemoteCurrentRevocation,
			),
			RemoteNextRevocation: PubKeyToString(
				channel.RemoteNextRevocation,
			),
			LocalChanConfig: ToChannelConfig(
				params, channel.LocalChanConfig,
			),
		}
	}
	return dumpChannels, nil
}

// BackupDump converts the given multi backup into a dumpable format.
func BackupDump(multi *chanbackup.Multi, params *chaincfg.Params) []BackupSingle {

	dumpSingles := make([]BackupSingle, len(multi.StaticBackups))
	for idx, single := range multi.StaticBackups {
		dumpSingles[idx] = BackupSingle{
			Version:         single.Version,
			IsInitiator:     single.IsInitiator,
			ChainHash:       single.ChainHash.String(),
			FundingOutpoint: single.FundingOutpoint.String(),
			ShortChannelID:  single.ShortChannelID,
			RemoteNodePub: PubKeyToString(
				single.RemoteNodePub,
			),
			Addresses: single.Addresses,
			Capacity:  single.Capacity,
			LocalChanCfg: ToChannelConfig(
				params, single.LocalChanCfg,
			),
			RemoteChanCfg: ToChannelConfig(
				params, single.RemoteChanCfg,
			),
			ShaChainRootDesc: ToKeyDescriptor(
				params, single.ShaChainRootDesc,
			),
		}
	}
	return dumpSingles
}

func ToChannelConfig(params *chaincfg.Params,
	cfg channeldb.ChannelConfig) ChannelConfig {

	return ChannelConfig{
		ChannelConstraints: cfg.ChannelConstraints,
		MultiSigKey:        ToKeyDescriptor(params, cfg.MultiSigKey),
		RevocationBasePoint: ToKeyDescriptor(
			params, cfg.RevocationBasePoint,
		),
		PaymentBasePoint: ToKeyDescriptor(
			params, cfg.PaymentBasePoint,
		),
		DelayBasePoint: ToKeyDescriptor(
			params, cfg.DelayBasePoint,
		),
		HtlcBasePoint: ToKeyDescriptor(params, cfg.HtlcBasePoint),
	}
}

func ToKeyDescriptor(params *chaincfg.Params,
	desc keychain.KeyDescriptor) KeyDescriptor {

	return KeyDescriptor{
		Path: fmt.Sprintf(
			lndInternalDerivationPath, params.HDCoinType,
			desc.Family, desc.Index,
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
