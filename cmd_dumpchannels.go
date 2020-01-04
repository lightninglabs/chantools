package chantools

import (
	"bytes"
	"encoding/hex"

	"github.com/davecgh/go-spew/spew"
	"github.com/guggero/chantools/dump"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
)

func dumpChannelInfo(chanDb *channeldb.DB) error {
	channels, err := chanDb.FetchAllChannels()
	if err != nil {
		return err
	}

	dumpChannels := make([]dump.OpenChannel, len(channels))
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

		dumpChannels[idx] = dump.OpenChannel{
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
			IdentityPub: dump.PubKeyToString(
				channel.IdentityPub,
			),
			Capacity:          channel.Capacity,
			TotalMSatSent:     channel.TotalMSatSent,
			TotalMSatReceived: channel.TotalMSatReceived,
			PerCommitPoint:    dump.PubKeyToString(perCommitPoint),
			LocalChanCfg: dump.ToChannelConfig(
				channel.LocalChanCfg,
			),
			RemoteChanCfg: dump.ToChannelConfig(
				channel.RemoteChanCfg,
			),
			LocalCommitment:  channel.LocalCommitment,
			RemoteCommitment: channel.RemoteCommitment,
			RemoteCurrentRevocation: dump.PubKeyToString(
				channel.RemoteCurrentRevocation,
			),
			RemoteNextRevocation: dump.PubKeyToString(
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
