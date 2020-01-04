package dataformat

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/lightningnetwork/lnd/channeldb"
	"strconv"
	"strings"
)

type NumberString uint64

func (n *NumberString) UnmarshalJSON(b []byte) error {
	if b[0] != '"' {
		return json.Unmarshal(b, (*uint64)(n))
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	*n = NumberString(i)
	return nil
}

type InputFile interface {
	AsSummaryEntries() ([]*SummaryEntry, error)
}

type Input interface {
	AsSummaryEntry() *SummaryEntry
}

type ListChannelsFile struct {
	Channels []*ListChannelsChannel `json:"channels"`
}

func (f *ListChannelsFile) AsSummaryEntries() ([]*SummaryEntry, error) {
	result := make([]*SummaryEntry, len(f.Channels))
	for idx, entry := range f.Channels {
		result[idx] = entry.AsSummaryEntry()
	}
	return result, nil
}

type ListChannelsChannel struct {
	RemotePubkey  string       `json:"remote_pubkey"`
	ChannelPoint  string       `json:"channel_point"`
	Capacity      NumberString `json:"capacity"`
	Initiator     bool         `json:"initiator"`
	LocalBalance  NumberString `json:"local_balance"`
	RemoteBalance NumberString `json:"remote_balance"`
}

func (c *ListChannelsChannel) AsSummaryEntry() *SummaryEntry {
	return &SummaryEntry{
		RemotePubkey:   c.RemotePubkey,
		ChannelPoint:   c.ChannelPoint,
		FundingTXID:    fundingTXID(c.ChannelPoint),
		FundingTXIndex: fundingTXIndex(c.ChannelPoint),
		Capacity:       uint64(c.Capacity),
		Initiator:      c.Initiator,
		LocalBalance:   uint64(c.LocalBalance),
		RemoteBalance:  uint64(c.RemoteBalance),
	}
}

type PendingChannelsFile struct {
	PendingOpen         []*PendingChannelsChannel `json:"pending_open_channels"`
	PendingClosing      []*PendingChannelsChannel `json:"pending_closing_channels"`
	PendingForceClosing []*PendingChannelsChannel `json:"pending_force_closing_channels"`
	WaitingClose        []*PendingChannelsChannel `json:"waiting_close_channels"`
}

func (f *PendingChannelsFile) AsSummaryEntries() ([]*SummaryEntry, error) {
	numChannels := len(f.PendingOpen) + len(f.PendingClosing) +
		len(f.PendingForceClosing) + len(f.WaitingClose)
	result := make([]*SummaryEntry, numChannels)
	idx := 0
	for _, entry := range f.PendingOpen {
		result[idx] = entry.AsSummaryEntry()
		idx++
	}
	for _, entry := range f.PendingClosing {
		result[idx] = entry.AsSummaryEntry()
		idx++
	}
	for _, entry := range f.PendingForceClosing {
		result[idx] = entry.AsSummaryEntry()
		idx++
	}
	for _, entry := range f.WaitingClose {
		result[idx] = entry.AsSummaryEntry()
		idx++
	}
	return result, nil
}

type PendingChannelsChannel struct {
	Channel struct {
		RemotePubkey  string       `json:"remote_node_pub"`
		ChannelPoint  string       `json:"channel_point"`
		Capacity      NumberString `json:"capacity"`
		LocalBalance  NumberString `json:"local_balance"`
		RemoteBalance NumberString `json:"remote_balance"`
	} `json:"channel"`
}

func (c *PendingChannelsChannel) AsSummaryEntry() *SummaryEntry {
	return &SummaryEntry{
		RemotePubkey:   c.Channel.RemotePubkey,
		ChannelPoint:   c.Channel.ChannelPoint,
		FundingTXID:    fundingTXID(c.Channel.ChannelPoint),
		FundingTXIndex: fundingTXIndex(c.Channel.ChannelPoint),
		Capacity:       uint64(c.Channel.Capacity),
		Initiator:      false,
		LocalBalance:   uint64(c.Channel.LocalBalance),
		RemoteBalance:  uint64(c.Channel.RemoteBalance),
	}
}

type ChannelDBFile struct {
	DB *channeldb.DB
}

func (c *ChannelDBFile) AsSummaryEntries() ([]*SummaryEntry, error) {
	channels, err := c.DB.FetchAllChannels()
	if err != nil {
		return nil, fmt.Errorf("error fetching channels: %v", err)
	}
	result := make([]*SummaryEntry, len(channels))
	for idx, channel := range channels {
		result[idx] = &SummaryEntry{
			RemotePubkey: hex.EncodeToString(
				channel.IdentityPub.SerializeCompressed(),
			),
			ChannelPoint:   channel.FundingOutpoint.String(),
			FundingTXID:    channel.FundingOutpoint.Hash.String(),
			FundingTXIndex: channel.FundingOutpoint.Index,
			Capacity:       uint64(channel.Capacity),
			Initiator:      channel.IsInitiator,
			LocalBalance: uint64(
				channel.LocalCommitment.LocalBalance.ToSatoshis(),
			),
			RemoteBalance: uint64(
				channel.LocalCommitment.RemoteBalance.ToSatoshis(),
			),
		}
	}
	return result, nil
}

func (f *SummaryEntryFile) AsSummaryEntries() ([]*SummaryEntry, error) {
	return f.Channels, nil
}

func fundingTXID(chanPoint string) string {
	parts := strings.Split(chanPoint, ":")
	if len(parts) != 2 {
		panic(fmt.Errorf("channel point not in format <txid>:<idx>: %s",
			chanPoint))
	}
	return parts[0]
}

func fundingTXIndex(chanPoint string) uint32 {
	parts := strings.Split(chanPoint, ":")
	if len(parts) != 2 {
		panic(fmt.Errorf("channel point not in format <txid>:<idx>",
			chanPoint))
	}
	return uint32(parseInt(parts[1]))
}

func parseInt(str string) uint64 {
	index, err := strconv.Atoi(str)
	if err != nil {
		panic(fmt.Errorf("error parsing '%s' as int: %v", str, err))
	}
	return uint64(index)
}
