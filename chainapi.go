package chansummary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type chainApi struct {
	baseUrl string
}

type transaction struct {
	Vin  []*vin  `json:"vin"`
	Vout []*vout `json:"vout"`
}

type vin struct {
	Tixid   string `json:"txid"`
	Vout    int    `json:"vout"`
	Prevout *vout  `json:"prevout"`
}

type vout struct {
	ScriptPubkey     string `json:"scriptpubkey"`
	ScriptPubkeyAsm  string `json:"scriptpubkey_asm"`
	ScriptPubkeyType string `json:"scriptpubkey_type"`
	ScriptPubkeyAddr string `json:"scriptpubkey_addr"`
	Value            uint64  `json:"value"`
	outspend         *outspend
}

type outspend struct {
	Spent  bool    `json:"spent"`
	Txid   string  `json:"txid"`
	Vin    int     `json:"vin"`
	Status *status `json:"status"`
}

type status struct {
	Confirmed   bool   `json:"confirmed"`
	BlockHeight int    `json:"block_height"`
	BlockHash   string `json:"block_hash"`
}

func (a *chainApi) Transaction(txid string) (*transaction, error) {
	tx := &transaction{}
	err := Fetch(fmt.Sprintf("%s/tx/%s", a.baseUrl, txid), tx)
	if err != nil {
		return nil, err
	}
	for idx, vout := range tx.Vout {
		url := fmt.Sprintf(
			"%s/tx/%s/outspend/%d", a.baseUrl, txid, idx,
		)
		outspend := outspend{}
		err := Fetch(url, &outspend)
		if err != nil {
			return nil, err
		}
		vout.outspend = &outspend
	}
	return tx, nil
}

func Fetch(url string, target interface{}) error {
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
	return json.Unmarshal(body.Bytes(), target)
}
