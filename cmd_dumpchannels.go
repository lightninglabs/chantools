package chantools

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
	"github.com/davecgh/go-spew/spew"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwire"
)

const (
	lndInternalDerivationPath = "m/1017'/0'/%d'/0/%d"
)

// dumpInfo is the information we want to dump from an open channel in lnd's
// channel DB. See `channeldb.OpenChannel` for information about the fields.
type dumpInfo struct {
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
	LocalChanCfg            dumpChanCfg
	RemoteChanCfg           dumpChanCfg
	LocalCommitment         channeldb.ChannelCommitment
	RemoteCommitment        channeldb.ChannelCommitment
	RemoteCurrentRevocation string
	RemoteNextRevocation    string
	FundingTxn              string
	LocalShutdownScript     lnwire.DeliveryAddress
	RemoteShutdownScript    lnwire.DeliveryAddress
}

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

func dumpChannelInfo(chanDb *channeldb.DB) error {
	channels, err := chanDb.FetchAllChannels()
	if err != nil {
		return err
	}

	dumpChannels := make([]dumpInfo, len(channels))
	for idx, channel := range channels {
		var buf bytes.Buffer
		if channel.FundingTxn != nil {
			err = channel.FundingTxn.Serialize(&buf)
			if err != nil {
				return err
			}
		}
		revPreimage, err := channel.RevocationProducer.AtIndex(
			channel.LocalCommitment.CommitHeight,
		)
		if err != nil {
			return err
		}
		perCommitPoint := input.ComputeCommitmentPoint(revPreimage[:])

		dumpChannels[idx] = dumpInfo{
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
			IdentityPub: pubKeyToString(
				channel.IdentityPub,
			),
			Capacity:          channel.Capacity,
			TotalMSatSent:     channel.TotalMSatSent,
			TotalMSatReceived: channel.TotalMSatReceived,
			PerCommitPoint:    pubKeyToString(perCommitPoint),
			LocalChanCfg:      toDumpChanCfg(channel.LocalChanCfg),
			RemoteChanCfg:     toDumpChanCfg(channel.RemoteChanCfg),
			LocalCommitment:   channel.LocalCommitment,
			RemoteCommitment:  channel.RemoteCommitment,
			RemoteCurrentRevocation: pubKeyToString(
				channel.RemoteCurrentRevocation,
			),
			RemoteNextRevocation: pubKeyToString(
				channel.RemoteNextRevocation,
			),
			FundingTxn:           hex.EncodeToString(buf.Bytes()),
			LocalShutdownScript:  channel.LocalShutdownScript,
			RemoteShutdownScript: channel.RemoteShutdownScript,
		}
	}

	spew.Dump(dumpChannels)
	return nil
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
	return hex.EncodeToString(pubkey.SerializeCompressed())
}
