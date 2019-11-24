package chantools

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/lightningnetwork/lnd/channeldb"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

type InputFile interface {
	AsSummaryEntries() ([]*SummaryEntry, error)
}

type Input interface {
	AsSummaryEntry() *SummaryEntry
}

func ParseInput(cfg *config) ([]*SummaryEntry, error) {
	var (
		content []byte
		err     error
		target  InputFile
	)

	switch {
	case cfg.ListChannels != "":
		content, err = readInput(cfg.ListChannels)
		target = &listChannelsFile{}

	case cfg.PendingChannels != "":
		content, err = readInput(cfg.PendingChannels)
		target = &pendingChannelsFile{}

	case cfg.FromSummary != "":
		content, err = readInput(cfg.FromSummary)
		target = &SummaryEntryFile{}

	case cfg.FromChannelDB != "":
		db, err := channeldb.Open(cfg.FromChannelDB)
		if err != nil {
			return nil, fmt.Errorf("error opening channel DB: %v",
				err)
		}
		target = &channelDBFile{db: db}
		return target.AsSummaryEntries()

	default:
		return nil, fmt.Errorf("an input file must be specified")
	}

	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	err = decoder.Decode(&target)
	if err != nil {
		return nil, err
	}
	return target.AsSummaryEntries()
}

func readInput(input string) ([]byte, error) {
	if strings.TrimSpace(input) == "-" {
		return ioutil.ReadAll(os.Stdin)
	}
	return ioutil.ReadFile(input)
}

type listChannelsFile struct {
	Channels []*listChannelsChannel `json:"channels"`
}

func (f *listChannelsFile) AsSummaryEntries() ([]*SummaryEntry, error) {
	result := make([]*SummaryEntry, len(f.Channels))
	for idx, entry := range f.Channels {
		result[idx] = entry.AsSummaryEntry()
	}
	return result, nil
}

type listChannelsChannel struct {
	RemotePubkey     string `json:"remote_pubkey"`
	ChannelPoint     string `json:"channel_point"`
	CapacityStr      string `json:"capacity"`
	Initiator        bool   `json:"initiator"`
	LocalBalanceStr  string `json:"local_balance"`
	RemoteBalanceStr string `json:"remote_balance"`
}

func (c *listChannelsChannel) AsSummaryEntry() *SummaryEntry {
	return &SummaryEntry{
		RemotePubkey:   c.RemotePubkey,
		ChannelPoint:   c.ChannelPoint,
		FundingTXID:    fundingTXID(c.ChannelPoint),
		FundingTXIndex: fundingTXIndex(c.ChannelPoint),
		Capacity:       uint64(parseInt(c.CapacityStr)),
		Initiator:      c.Initiator,
		LocalBalance:   uint64(parseInt(c.LocalBalanceStr)),
		RemoteBalance:  uint64(parseInt(c.RemoteBalanceStr)),
	}
}

type pendingChannelsFile struct {
	PendingOpen         []*pendingChannelsChannel `json:"pending_open_channels"`
	PendingClosing      []*pendingChannelsChannel `json:"pending_closing_channels"`
	PendingForceClosing []*pendingChannelsChannel `json:"pending_force_closing_channels"`
	WaitingClose        []*pendingChannelsChannel `json:"waiting_close_channels"`
}

func (f *pendingChannelsFile) AsSummaryEntries() ([]*SummaryEntry, error) {
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

type pendingChannelsChannel struct {
	Channel struct {
		RemotePubkey     string `json:"remote_node_pub"`
		ChannelPoint     string `json:"channel_point"`
		CapacityStr      string `json:"capacity"`
		LocalBalanceStr  string `json:"local_balance"`
		RemoteBalanceStr string `json:"remote_balance"`
	} `json:"channel"`
}

func (c *pendingChannelsChannel) AsSummaryEntry() *SummaryEntry {
	return &SummaryEntry{
		RemotePubkey:   c.Channel.RemotePubkey,
		ChannelPoint:   c.Channel.ChannelPoint,
		FundingTXID:    fundingTXID(c.Channel.ChannelPoint),
		FundingTXIndex: fundingTXIndex(c.Channel.ChannelPoint),
		Capacity:       uint64(parseInt(c.Channel.CapacityStr)),
		Initiator:      false,
		LocalBalance:   uint64(parseInt(c.Channel.LocalBalanceStr)),
		RemoteBalance:  uint64(parseInt(c.Channel.RemoteBalanceStr)),
	}
}

type channelDBFile struct {
	db *channeldb.DB
}

func (c *channelDBFile) AsSummaryEntries() ([]*SummaryEntry, error) {
	channels, err := c.db.FetchAllChannels()
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
			LocalBalance:   uint64(
				channel.LocalCommitment.LocalBalance.ToSatoshis(),
			),
			RemoteBalance:  uint64(
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

func parseInt(str string) int {
	index, err := strconv.Atoi(str)
	if err != nil {
		panic(fmt.Errorf("error parsing '%s' as int: %v", str, err))
	}
	return index
}
