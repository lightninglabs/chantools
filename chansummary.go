package chansummary

import (
	"fmt"
	"strconv"
	"strings"
)

type channel struct {
	RemotePubkey  string `json:"remote_pubkey"`
	ChannelPoint  string `json:"channel_point"`
	Capacity      string `json:"capacity"`
	Initiator     bool   `json:"initiator"`
	LocalBalance  string `json:"local_balance"`
	RemoteBalance string `json:"remote_balance"`
}

func (c *channel) FundingTXID() string {
	parts := strings.Split(c.ChannelPoint, ":")
	if len(parts) != 2 {
		panic(fmt.Errorf("channel point not in format <txid>:<idx>"))
	}
	return parts[0]
}

func (c *channel) FundingTXIndex() int {
	parts := strings.Split(c.ChannelPoint, ":")
	if len(parts) != 2 {
		panic(fmt.Errorf("channel point not in format <txid>:<idx>"))
	}
	return parseInt(parts[1])
}

func (c *channel) localBalance() uint64 {
	return uint64(parseInt(c.LocalBalance))
}

func (c *channel) remoteBalance() uint64 {
	return uint64(parseInt(c.RemoteBalance))
}

func collectChanSummary(cfg *config, channels []*channel) error {
	var (
		chansClosed  = 0
		chansOpen    = 0
		valueUnspent = uint64(0)
		valueSalvage = uint64(0)
		valueSafe    = uint64(0)
	)

	chainApi := &chainApi{baseUrl: cfg.ApiUrl}

	for idx, channel := range channels {
		tx, err := chainApi.Transaction(channel.FundingTXID())
		if err != nil {
			return err
		}
		outspend := tx.Vout[channel.FundingTXIndex()].outspend
		if outspend.Spent {
			chansClosed++

			s, f, err := reportOutspend(chainApi, channel, outspend)
			if err != nil {
				return err
			}
			valueSalvage += s
			valueSafe += f
		} else {
			chansOpen++
			valueUnspent += channel.localBalance()
		}

		if idx%50 == 0 {
			fmt.Printf("Queried channel %d of %d.\n", idx,
				len(channels))
		}
	}

	fmt.Printf("Finished scanning.\nClosed channels: %d\nOpen channels: "+
		"%d\nSats in open channels: %d\nSats that can possibly be "+
		"salvaged: %d\nSats in co-op close channels: %d\n", chansClosed,
		chansOpen, valueUnspent, valueSalvage, valueSafe)

	return nil
}

func reportOutspend(api *chainApi, ch *channel, os *outspend) (uint64, uint64,
	error) {

	spendTx, err := api.Transaction(os.Txid)
	if err != nil {
		return 0, 0, err
	}

	numSpent := 0
	salvageBalance := uint64(0)
	safeBalance := uint64(0)
	for _, vout := range spendTx.Vout {
		if vout.outspend.Spent {
			numSpent++
		}
	}
	if numSpent != len(spendTx.Vout) {
		fmt.Printf("Channel %s spent by %s:%d which has %d outputs of "+
			"which %d are spent:\n", ch.ChannelPoint, os.Txid,
			os.Vin, len(spendTx.Vout), numSpent)
		var utxo []*vout
		for _, vout := range spendTx.Vout {
			if !vout.outspend.Spent {
				utxo = append(utxo, vout)
			}
		}

		if salvageable(ch, utxo) {
			salvageBalance += utxo[0].Value

			outs := spendTx.Vout

			switch {
			case len(outs) == 1 &&
				outs[0].ScriptPubkeyType == "v0_p2wpkh" &&
				outs[0].outspend.Spent == false:

				safeBalance += utxo[0].Value

			case len(outs) == 2 &&
				outs[0].ScriptPubkeyType == "v0_p2wpkh" &&
				outs[1].ScriptPubkeyType == "v0_p2wpkh":

				safeBalance += utxo[0].Value
			}
		} else {
			for idx, vout := range spendTx.Vout {
				if !vout.outspend.Spent {
					fmt.Printf("UTXO %d of type %s with "+
						"value %d\n", idx,
						vout.ScriptPubkeyType,
						vout.Value)
				}
			}
			fmt.Printf("Local balance: %s\n", ch.LocalBalance)
			fmt.Printf("Remote balance: %s\n", ch.RemoteBalance)
			fmt.Printf("Initiator: %v\n", ch.Initiator)
		}

	}

	return salvageBalance, safeBalance, nil
}

func salvageable(ch *channel, utxo []*vout) bool {
	return ch.localBalance() == utxo[0].Value ||
		ch.remoteBalance() == 0
}

func parseInt(str string) int {
	index, err := strconv.Atoi(str)
	if err != nil {
		panic(fmt.Errorf("error parsing '%s' as int: %v", str, err))
	}
	return index
}
