package chantools

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/guggero/chantools/chain"
	"github.com/guggero/chantools/dataformat"
)

func summarizeChannels(apiUrl string,
	channels []*dataformat.SummaryEntry) error {

	summaryFile := &dataformat.SummaryEntryFile{
		Channels: channels,
	}
	chainApi := &chain.Api{BaseUrl: apiUrl}

	for idx, channel := range channels {
		tx, err := chainApi.Transaction(channel.FundingTXID)
		if err == chain.ErrTxNotFound {
			log.Errorf("Funding TX %s not found. Ignoring.",
				channel.FundingTXID)
			channel.ChanExists = false
			continue
		}
		if err != nil {
			log.Errorf("Problem with channel %d (%s): %v.",
				idx, channel.FundingTXID, err)
			return err
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
				chainApi, summaryFile, channel, outspend,
			)
			if err != nil {
				log.Errorf("Problem with channel %d (%s): %v.",
					idx, channel.FundingTXID, err)
				return err
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

	log.Info("Finished scanning.")
	log.Infof("Open channels: %d", summaryFile.OpenChannels)
	log.Infof("Sats in open channels: %d", summaryFile.FundsOpenChannels)
	log.Infof("Closed channels: %d", summaryFile.ClosedChannels)
	log.Infof(" --> force closed channels: %d",
		summaryFile.ForceClosedChannels)
	log.Infof(" --> coop closed channels: %d",
		summaryFile.CoopClosedChannels)
	log.Infof(" --> closed channels with all outputs spent: %d",
		summaryFile.FullySpentChannels)
	log.Infof(" --> closed channels with unspent outputs: %d",
		summaryFile.ChannelsWithUnspent)
	log.Infof(" --> closed channels with potentially our outputs: %d",
		summaryFile.ChannelsWithPotential)
	log.Infof("Sats in closed channels: %d", summaryFile.FundsClosedChannels)
	log.Infof(" --> closed channel sats that have been swept/spent: %d",
		summaryFile.FundsClosedSpent)
	log.Infof(" --> closed channel sats that are in force-close outputs: %d",
		summaryFile.FundsForceClose)
	log.Infof(" --> closed channel sats that are in coop close outputs: %d",
		summaryFile.FundsCoopClose)

	summaryBytes, err := json.MarshalIndent(summaryFile, "", " ")
	if err != nil {
		return err
	}
	fileName := fmt.Sprintf("results/summary-%s.json",
		time.Now().Format("2006-01-02-15-04-05"))
	log.Infof("Writing result to %s", fileName)
	return ioutil.WriteFile(fileName, summaryBytes, 0644)
}

func reportOutspend(api *chain.Api, summaryFile *dataformat.SummaryEntryFile,
	entry *dataformat.SummaryEntry, os *chain.Outspend) error {

	spendTx, err := api.Transaction(os.Txid)
	if err != nil {
		return err
	}

	summaryFile.FundsClosedChannels += entry.LocalBalance
	var utxo []*chain.Vout
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

		if couldBeOurs(entry, utxo) {
			summaryFile.ChannelsWithPotential++
			summaryFile.FundsForceClose += utxo[0].Value
			entry.HasPotential = true

			// Could maybe be brute forced.
			if len(utxo) == 1 &&
				utxo[0].ScriptPubkeyType == "v0_p2wpkh" &&
				utxo[0].Outspend.Spent == false {

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

func couldBeOurs(entry *dataformat.SummaryEntry, utxo []*chain.Vout) bool {
	if len(utxo) == 1 && utxo[0].Value == entry.RemoteBalance {
		return false
	}

	return entry.LocalBalance != 0
}

func isCoopClose(tx *chain.TX) bool {
	return tx.Vin[0].Sequence == 0xffffffff
}
