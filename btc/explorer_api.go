package btc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var (
	ErrTxNotFound = errors.New("transaction not found")
)

type ExplorerAPI struct {
	BaseURL string
}

type TX struct {
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

func (a *ExplorerAPI) PublishTx(rawTxHex string) (string, error) {
	url := fmt.Sprintf("%s/tx", a.BaseURL)
	resp, err := http.Post(url, "text/plain", strings.NewReader(rawTxHex))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body := new(bytes.Buffer)
	_, err = body.ReadFrom(resp.Body)
	if err != nil {
		return "", err
	}
	return body.String(), nil
}

func fetchJSON(url string, target interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body := new(bytes.Buffer)
	_, err = body.ReadFrom(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body.Bytes(), target)
	if err != nil {
		if body.String() == "Transaction not found" {
			return ErrTxNotFound
		}
	}
	return err
}
