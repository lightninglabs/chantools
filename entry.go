package chantools

type ClosingTX struct {
	TXID         string `json:"txid"`
	ForceClose   bool   `json:"force_close"`
	AllOutsSpent bool   `json:"all_outputs_spent"`
	OurAddr      string `json:"our_addr"`
	SweepPrivkey string `json:"sweep_privkey"`
}

type Basepoint struct {
	Family uint16 `json:"family"`
	Index  uint32 `json:"index"`
	Pubkey string `json:"pubkey"`
}

type Out struct {
	Script    string `json:"script"`
	ScriptAsm string `json:"script_asm"`
	Value     uint64 `json:"value"`
}

type ForceClose struct {
	TXID           string     `json:"txid"`
	Serialized     string     `json:"serialized"`
	CSVDelay       uint16     `json:"csv_delay"`
	DelayBasepoint *Basepoint `json:"delay_basepoint"`
	CommitPoint    string     `json:"commit_point"`
	Outs           []*Out     `json:"outs"`
}

type SummaryEntry struct {
	RemotePubkey   string      `json:"remote_pubkey"`
	ChannelPoint   string      `json:"channel_point"`
	FundingTXID    string      `json:"funding_txid"`
	FundingTXIndex uint32      `json:"funding_tx_index"`
	Capacity       uint64      `json:"capacity"`
	Initiator      bool        `json:"initiator"`
	LocalBalance   uint64      `json:"local_balance"`
	RemoteBalance  uint64      `json:"remote_balance"`
	ClosingTX      *ClosingTX  `json:"closing_tx,omitempty"`
	ForceClose     *ForceClose `json:"force_close"`
}

type SummaryEntryFile struct {
	Channels              []*SummaryEntry `json:"channels"`
	OpenChannels          uint32          `json:"open_channels"`
	ClosedChannels        uint32          `json:"closed_channels"`
	ForceClosedChannels   uint32          `json:"force_closed_channels"`
	CoopClosedChannels    uint32          `json:"coop_closed_channels"`
	FullySpentChannels    uint32          `json:"fully_spent_channels"`
	ChannelsWithPotential uint32          `json:"channels_with_potential_funds"`
	FundsOpenChannels     uint64          `json:"funds_open_channels"`
	FundsClosedChannels   uint64          `json:"funds_closed_channels"`
	FundsClosedSpent      uint64          `json:"funds_closed_channels_spent"`
	FundsForceClose       uint64          `json:"funds_force_closed_maybe_ours"`
	FundsCoopClose        uint64          `json:"funds_coop_closed_maybe_ours"`
}
