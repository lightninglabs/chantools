package chantools

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"
)

func collectChanSummary(cfg *config, channels []*SummaryEntry) error {
	summaryFile := &SummaryEntryFile{
		Channels: channels,
	}

	chainApi := &chainApi{baseUrl: cfg.ApiUrl}

	for idx, channel := range channels {
		tx, err := chainApi.Transaction(channel.FundingTXID)
		if err == ErrTxNotFound {
			log.Errorf("Funding TX %s not found. Ignoring.",
				channel.FundingTXID)
			continue
		}
		if err != nil {
			log.Errorf("Problem with channel %d (%s): %v.",
				idx, channel.FundingTXID, err)
			return err
		}
		outspend := tx.Vout[channel.FundingTXIndex].outspend
		if outspend.Spent {
			summaryFile.ClosedChannels++
			channel.ClosingTX = &ClosingTX{
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
			summaryFile.FundsOpenChannels += uint64(channel.LocalBalance)
			channel.ClosingTX = nil
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

func reportOutspend(api *chainApi, summaryFile *SummaryEntryFile,
	entry *SummaryEntry, os *outspend) error {

	spendTx, err := api.Transaction(os.Txid)
	if err != nil {
		return err
	}

	summaryFile.FundsClosedChannels += uint64(entry.LocalBalance)

	if isCoopClose(spendTx) {
		summaryFile.CoopClosedChannels++
		summaryFile.FundsCoopClose += uint64(entry.LocalBalance)
		entry.ClosingTX.ForceClose = false
		return nil
	}

	summaryFile.ForceClosedChannels++
	entry.ClosingTX.ForceClose = true

	var utxo []*vout
	for _, vout := range spendTx.Vout {
		if !vout.outspend.Spent {
			utxo = append(utxo, vout)
		}
	}
	if len(utxo) > 0 {
		log.Debugf("Channel %s spent by %s:%d which has %d outputs of "+
			"which %d are unspent.", entry.ChannelPoint, os.Txid,
			os.Vin, len(spendTx.Vout), len(utxo))

		entry.ClosingTX.AllOutsSpent = false
		summaryFile.ChannelsWithPotential++

		if couldBeOurs(entry, utxo) {
			summaryFile.FundsForceClose += utxo[0].Value

			switch {
			case len(utxo) == 1 &&
				utxo[0].ScriptPubkeyType == "v0_p2wpkh" &&
				utxo[0].outspend.Spent == false:

				entry.ClosingTX.OurAddr = utxo[0].ScriptPubkeyAddr
			}
		} else {
			for idx, vout := range spendTx.Vout {
				if !vout.outspend.Spent {
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
		summaryFile.FundsClosedSpent += uint64(entry.LocalBalance)
		summaryFile.FullySpentChannels++
	}

	return nil
}

func couldBeOurs(entry *SummaryEntry, utxo []*vout) bool {
	return utxo[0].ScriptPubkeyType == "v0_p2wpkh" && entry.LocalBalance != 0
}

func isCoopClose(tx *transaction) bool {
	return tx.Vin[0].Sequence == 0xffffffff
}
