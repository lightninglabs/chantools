package btc

import (
	"errors"

	"github.com/btcsuite/btclog"
	"github.com/lightninglabs/chantools/dataformat"
)

func SummarizeChannels(apiURL string, channels []*dataformat.SummaryEntry,
	log btclog.Logger) (*dataformat.SummaryEntryFile, error) {

	summaryFile := &dataformat.SummaryEntryFile{
		Channels: channels,
	}
	api := &ExplorerAPI{BaseURL: apiURL}

	for idx, channel := range channels {
		tx, err := api.Transaction(channel.FundingTXID)
		if errors.Is(err, ErrTxNotFound) {
			log.Errorf("Funding TX %s not found. Ignoring.",
				channel.FundingTXID)
			channel.ChanExists = false
			continue
		}
		if err != nil {
			log.Errorf("Problem with channel %d (%s): %v.",
				idx, channel.FundingTXID, err)
			return nil, err
		}
		channel.ChanExists = true
		outspend := tx.Vout[channel.FundingTXIndex].Outspend
		if outspend.Spent {
			summaryFile.ClosedChannels++
			channel.ClosingTX = &dataformat.ClosingTX{
				TXID:       outspend.Txid,
				ConfHeight: uint32(outspend.Status.BlockHeight),
			}

			err := reportOutspend(
				api, summaryFile, channel, outspend, log,
			)
			if err != nil {
				log.Errorf("Problem with channel %d (%s): %v.",
					idx, channel.FundingTXID, err)
				return nil, err
			}
		} else {
			summaryFile.OpenChannels++
			summaryFile.FundsOpenChannels += channel.LocalBalance
			channel.ClosingTX = nil
			channel.HasPotential = true
		}

		if idx%50 == 0 {
			log.Infof("Queried channel %d of %d.", idx,
				len(channels))
		}
	}

	return summaryFile, nil
}

func reportOutspend(api *ExplorerAPI,
	summaryFile *dataformat.SummaryEntryFile,
	entry *dataformat.SummaryEntry, os *Outspend, log btclog.Logger) error {

	spendTx, err := api.Transaction(os.Txid)
	if err != nil {
		return err
	}

	summaryFile.FundsClosedChannels += entry.LocalBalance
	var utxo []*Vout
	for _, vout := range spendTx.Vout {
		if !vout.Outspend.Spent {
			utxo = append(utxo, vout)
		}
	}

	if isCoopClose(spendTx) {
		summaryFile.CoopClosedChannels++
		summaryFile.FundsCoopClose += entry.LocalBalance
		entry.ClosingTX.ForceClose = false
		entry.ClosingTX.AllOutsSpent = len(utxo) == 0
		entry.HasPotential = entry.LocalBalance > 0 && len(utxo) != 0
		return nil
	}

	summaryFile.ForceClosedChannels++
	entry.ClosingTX.ForceClose = true
	entry.HasPotential = false

	if len(utxo) > 0 {
		log.Debugf("Channel %s spent by %s:%d which has %d outputs of "+
			"which %d are unspent.", entry.ChannelPoint, os.Txid,
			os.Vin, len(spendTx.Vout), len(utxo))

		entry.ClosingTX.AllOutsSpent = false
		summaryFile.ChannelsWithUnspent++

		for _, o := range utxo {
			if o.ScriptPubkeyType == "v0_p2wpkh" {
				entry.ClosingTX.ToRemoteAddr = o.ScriptPubkeyAddr
			}
		}

		if couldBeOurs(entry, utxo) {
			summaryFile.ChannelsWithPotential++
			summaryFile.FundsForceClose += utxo[0].Value
			entry.HasPotential = true

			// Could maybe be brute forced.
			if len(utxo) == 1 &&
				utxo[0].ScriptPubkeyType == "v0_p2wpkh" &&
				!utxo[0].Outspend.Spent {

				entry.ClosingTX.OurAddr = utxo[0].ScriptPubkeyAddr
			}
		} else {
			// It's theirs, ignore.
			if entry.LocalBalance == 0 ||
				(len(utxo) == 1 &&
					utxo[0].Value == entry.RemoteBalance) {

				return nil
			}

			// We don't know what this output is, logging for debug.
			for idx, vout := range spendTx.Vout {
				if !vout.Outspend.Spent {
					log.Debugf("UTXO %d of type %s with "+
						"value %d", idx,
						vout.ScriptPubkeyType,
						vout.Value)
				}
			}
			log.Debugf("Local balance: %d", entry.LocalBalance)
			log.Debugf("Remote balance: %d", entry.RemoteBalance)
			log.Debugf("Initiator: %v", entry.Initiator)
		}
	} else {
		entry.ClosingTX.AllOutsSpent = true
		entry.HasPotential = false
		summaryFile.FundsClosedSpent += entry.LocalBalance
		summaryFile.FullySpentChannels++
	}

	return nil
}

func couldBeOurs(entry *dataformat.SummaryEntry, utxo []*Vout) bool {
	if len(utxo) == 1 && utxo[0].Value == entry.RemoteBalance {
		return false
	}

	return entry.LocalBalance != 0
}

func isCoopClose(tx *TX) bool {
	return tx.Vin[0].Sequence == 0xffffffff
}
