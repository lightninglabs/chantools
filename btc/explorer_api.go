package btc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var (
	ErrTxNotFound = errors.New("transaction not found")
)

type ExplorerAPI struct {
	BaseURL string
}

type TX struct {
	TXID string  `json:"txid"`
	Vin  []*Vin  `json:"vin"`
	Vout []*Vout `json:"vout"`
}

type Vin struct {
	Tixid    string `json:"txid"`
	Vout     int    `json:"vout"`
	Prevout  *Vout  `json:"prevout"`
	Sequence uint32 `json:"sequence"`
}

type Vout struct {
	ScriptPubkey     string `json:"scriptpubkey"`
	ScriptPubkeyAsm  string `json:"scriptpubkey_asm"`
	ScriptPubkeyType string `json:"scriptpubkey_type"`
	ScriptPubkeyAddr string `json:"scriptpubkey_address"`
	Value            uint64 `json:"value"`
	Outspend         *Outspend
}

type Outspend struct {
	Spent  bool    `json:"spent"`
	Txid   string  `json:"txid"`
	Vin    int     `json:"vin"`
	Status *Status `json:"status"`
}

type Status struct {
	Confirmed   bool   `json:"confirmed"`
	BlockHeight int    `json:"block_height"`
	BlockHash   string `json:"block_hash"`
}

type Stats struct {
	FundedTXOCount uint32 `json:"funded_txo_count"`
	FundedTXOSum   uint64 `json:"funded_txo_sum"`
	SpentTXOCount  uint32 `json:"spent_txo_count"`
	SpentTXOSum    uint64 `json:"spent_txo_sum"`
	TXCount        uint32 `json:"tx_count"`
}

type AddressStats struct {
	Address      string `json:"address"`
	ChainStats   *Stats `json:"chain_stats"`
	MempoolStats *Stats `json:"mempool_stats"`
}

func (a *ExplorerAPI) Transaction(txid string) (*TX, error) {
	tx := &TX{}
	err := fetchJSON(fmt.Sprintf("%s/tx/%s", a.BaseURL, txid), tx)
	if err != nil {
		return nil, err
	}
	for idx, vout := range tx.Vout {
		url := fmt.Sprintf(
			"%s/tx/%s/outspend/%d", a.BaseURL, txid, idx,
		)
		outspend := Outspend{}
		err := fetchJSON(url, &outspend)
		if err != nil {
			return nil, err
		}
		vout.Outspend = &outspend
	}
	return tx, nil
}

func (a *ExplorerAPI) Outpoint(addr string) (*TX, int, error) {
	var txs []*TX
	err := fetchJSON(
		fmt.Sprintf("%s/address/%s/txs", a.BaseURL, addr), &txs,
	)
	if err != nil {
		return nil, 0, err
	}
	for _, tx := range txs {
		for idx, vout := range tx.Vout {
			if vout.ScriptPubkeyAddr == addr {
				return tx, idx, nil
			}
		}
	}

	return nil, 0, fmt.Errorf("no tx found")
}

func (a *ExplorerAPI) Spends(addr string) ([]*TX, error) {
	var txs []*TX
	err := fetchJSON(
		fmt.Sprintf("%s/address/%s/txs", a.BaseURL, addr), &txs,
	)
	if err != nil {
		return nil, err
	}

	var spends []*TX
	for txIndex := range txs {
		tx := txs[txIndex]
		for _, vin := range tx.Vin {
			if vin.Prevout.ScriptPubkeyAddr == addr {
				spends = append(spends, tx)
			}
		}
	}

	return spends, nil
}

func (a *ExplorerAPI) Unspent(addr string) ([]*Vout, error) {
	var (
		stats   = &AddressStats{}
		outputs []*Vout
		txs     []*TX
		err     error
	)
	err = fetchJSON(fmt.Sprintf("%s/address/%s", a.BaseURL, addr), &stats)
	if err != nil {
		return nil, err
	}

	confirmedUnspent := stats.ChainStats.FundedTXOSum -
		stats.ChainStats.SpentTXOSum
	unconfirmedUnspent := stats.MempoolStats.FundedTXOSum -
		stats.MempoolStats.SpentTXOSum

	if confirmedUnspent+unconfirmedUnspent == 0 {
		return nil, nil
	}

	err = fetchJSON(fmt.Sprintf("%s/address/%s/txs", a.BaseURL, addr), &txs)
	if err != nil {
		return nil, err
	}
	for _, tx := range txs {
		for voutIdx, vout := range tx.Vout {
			if vout.ScriptPubkeyAddr == addr {
				// We need to also make sure that the tx is not
				// already spent before including it as unspent.
				//
				// NOTE: Somehow LND sometimes contructs
				// channels with the same keyfamily base hence
				// the same pubkey. Needs to be investigated on
				// the LND side.
				outSpend := &Outspend{
					Txid: tx.TXID,
					Vin:  voutIdx,
				}
				url := fmt.Sprintf(
					"%s/tx/%s/outspend/%d", a.BaseURL,
					tx.TXID, voutIdx,
				)
				err := fetchJSON(url, outSpend)
				if err != nil {
					return nil, err
				}

				if outSpend.Spent {
					continue
				}

				vout.Outspend = outSpend
				outputs = append(outputs, vout)
			}
		}
	}

	return outputs, nil
}

func (a *ExplorerAPI) Address(outpoint string) (string, error) {
	parts := strings.Split(outpoint, ":")

	if len(parts) != 2 {
		return "", fmt.Errorf("invalid outpoint: %v", outpoint)
	}

	tx, err := a.Transaction(parts[0])
	if err != nil {
		return "", err
	}

	vout, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", err
	}

	if len(tx.Vout) <= vout {
		return "", fmt.Errorf("invalid output index: %d", vout)
	}

	return tx.Vout[vout].ScriptPubkeyAddr, nil
}

func (a *ExplorerAPI) PublishTx(rawTxHex string) (string, error) {
	url := fmt.Sprintf("%s/tx", a.BaseURL)
	resp, err := http.Post(url, "text/plain", strings.NewReader(rawTxHex))
	if err != nil {
		return "", fmt.Errorf("error posting data to API '%s', "+
			"server might be experiencing temporary issues, try "+
			"again later; error details: %w", url, err)
	}
	defer resp.Body.Close()
	body := new(bytes.Buffer)
	_, err = body.ReadFrom(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error fetching data from API '%s', "+
			"server might be experiencing temporary issues, try "+
			"again later; error details: %w", url, err)
	}
	return body.String(), nil
}

func fetchJSON(url string, target interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error fetching data from API '%s', "+
			"server might be experiencing temporary issues, try "+
			"again later; error details: %w", url, err)
	}
	defer resp.Body.Close()

	body := new(bytes.Buffer)
	_, err = body.ReadFrom(resp.Body)
	if err != nil {
		return fmt.Errorf("error fetching data from API '%s', "+
			"server might be experiencing temporary issues, try "+
			"again later; error details: %w", url, err)
	}
	err = json.Unmarshal(body.Bytes(), target)
	if err != nil {
		if body.String() == "Transaction not found" {
			return ErrTxNotFound
		}

		return fmt.Errorf("error decoding data from API '%s', "+
			"server might be experiencing temporary issues, try "+
			"again later; error details: %w", url, err)
	}

	return nil
}
