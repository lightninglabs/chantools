package dataformat

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/keychain"
)

type ClosingTX struct {
	TXID         string `json:"txid"`
	ForceClose   bool   `json:"force_close"`
	AllOutsSpent bool   `json:"all_outputs_spent"`
	OurAddr      string `json:"our_addr"`
	ToRemoteAddr string `json:"to_remote_addr"`
	SweepPrivkey string `json:"sweep_privkey"`
	ConfHeight   uint32 `json:"conf_height"`
}

type BasePoint struct {
	Family uint16 `json:"family,omitempty"`
	Index  uint32 `json:"index,omitempty"`
	PubKey string `json:"pubkey"`
}

func (b *BasePoint) Desc() (*keychain.KeyDescriptor, error) {
	pubKeyHex, err := hex.DecodeString(b.PubKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding base point pubkey: %w",
			err)
	}
	pubKey, err := btcec.ParsePubKey(pubKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error parsing base point pubkey: %w",
			err)
	}

	return &keychain.KeyDescriptor{
		KeyLocator: keychain.KeyLocator{
			Family: keychain.KeyFamily(b.Family),
			Index:  b.Index,
		},
		PubKey: pubKey,
	}, nil
}

type Out struct {
	Script    string `json:"script"`
	ScriptAsm string `json:"script_asm"`
	Value     uint64 `json:"value"`
}

type ForceClose struct {
	TXID                string     `json:"txid"`
	Serialized          string     `json:"serialized"`
	CSVDelay            uint16     `json:"csv_delay"`
	DelayBasePoint      *BasePoint `json:"delay_basepoint"`
	RevocationBasePoint *BasePoint `json:"revocation_basepoint"`
	CommitPoint         string     `json:"commit_point"`
	Outs                []*Out     `json:"outs"`
}

type SummaryEntry struct {
	RemotePubkey              string      `json:"remote_pubkey"`
	ChannelPoint              string      `json:"channel_point"`
	FundingTXID               string      `json:"funding_txid"`
	FundingTXIndex            uint32      `json:"funding_tx_index"`
	Capacity                  uint64      `json:"capacity"`
	Initiator                 bool        `json:"initiator"`
	LocalBalance              uint64      `json:"local_balance"`
	RemoteBalance             uint64      `json:"remote_balance"`
	ChanExists                bool        `json:"chan_exists_onchain"`
	HasPotential              bool        `json:"has_potential_funds"`
	LocalUnrevokedCommitPoint string      `json:"local_unrevoked_commit_point"`
	ClosingTX                 *ClosingTX  `json:"closing_tx,omitempty"`
	ForceClose                *ForceClose `json:"force_close"`
}

type SummaryEntryFile struct {
	Channels              []*SummaryEntry `json:"channels"`
	OpenChannels          uint32          `json:"open_channels"`
	ClosedChannels        uint32          `json:"closed_channels"`
	ForceClosedChannels   uint32          `json:"force_closed_channels"`
	CoopClosedChannels    uint32          `json:"coop_closed_channels"`
	FullySpentChannels    uint32          `json:"fully_spent_channels"`
	ChannelsWithUnspent   uint32          `json:"channels_with_unspent_funds"`
	ChannelsWithPotential uint32          `json:"channels_with_potential_funds"`
	FundsOpenChannels     uint64          `json:"funds_open_channels"`
	FundsClosedChannels   uint64          `json:"funds_closed_channels"`
	FundsClosedSpent      uint64          `json:"funds_closed_channels_spent"`
	FundsForceClose       uint64          `json:"funds_force_closed_maybe_ours"`
	FundsCoopClose        uint64          `json:"funds_coop_closed_maybe_ours"`
	OpenChannelList       []*SummaryEntry `json:"open_channel_list"`
}

func ExtractSummaryFromDump(data string) ([]*SummaryEntry, error) {
	// Regex to match the data pattern.
	pattern := `(?ms)  ChanPoint: \(string\) \(len=\d+\) "(.*?)",.*?  ` +
		`RemotePub: \(string\) \(len=66\) "(.*?)",.*?  ` +
		`Capacity: \(btcutil\.Amount\) ([\d\.]+) BTC,.*?  ` +
		`LocalUnrevokedCommitPoint: \(string\) \(len=66\) "(.*?)"`
	re := regexp.MustCompile(pattern)

	var results []*SummaryEntry
	matches := re.FindAllStringSubmatch(data, -1)
	for _, match := range matches {
		if len(match) == 5 {
			chanPoint := strings.TrimSpace(match[1])
			parsedOP, err := wire.NewOutPointFromString(chanPoint)
			if err != nil {
				return nil, fmt.Errorf("unable to parse "+
					"outpoint: %w", err)
			}

			capacity, err := strconv.ParseFloat(match[3], 64)
			if err != nil {
				return nil, fmt.Errorf("unable to parse "+
					"capacity: %w", err)
			}

			results = append(results, &SummaryEntry{
				RemotePubkey:   match[2],
				ChannelPoint:   chanPoint,
				FundingTXID:    parsedOP.Hash.String(),
				FundingTXIndex: parsedOP.Index,
				Capacity: uint64(
					capacity * 1e8,
				),
				LocalUnrevokedCommitPoint: match[4],
			})
		}
	}

	return results, nil
}
