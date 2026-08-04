package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/btc"
)

// sweepHtlcTarget describes one on-chain HTLC output requested by the user.
type sweepHtlcTarget struct {
	// outpoint is the exact commitment transaction output to sweep.
	outpoint wire.OutPoint

	// fundingPoint is the channel funding outpoint spent by the close tx.
	fundingPoint wire.OutPoint

	// value is the HTLC output value in satoshis.
	value int64

	// pkScript is the HTLC output script pubkey.
	pkScript []byte

	// closeTx is the force-close transaction that created the HTLC output.
	closeTx *btc.TX
}

// fetchSweepHtlcTargets parses and loads all requested HTLC targets.
func fetchSweepHtlcTargets(api *btc.ExplorerAPI,
	outpoints string) ([]*sweepHtlcTarget, error) {

	parts := strings.Split(outpoints, ",")
	targets := make([]*sweepHtlcTarget, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		op, err := wire.NewOutPointFromString(part)
		if err != nil {
			return nil, fmt.Errorf("invalid outpoint %q: %w", part, err)
		}

		target, err := fetchSweepHtlcTarget(api, *op)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}

	if len(targets) == 0 {
		return nil, errors.New("no valid outpoints specified")
	}

	return targets, nil
}

// fetchSweepHtlcTarget loads one target outpoint from the chain API.
func fetchSweepHtlcTarget(api *btc.ExplorerAPI,
	outpoint wire.OutPoint) (*sweepHtlcTarget, error) {

	closeTx, err := api.Transaction(outpoint.Hash.String())
	if err != nil {
		return nil, fmt.Errorf("error fetching close tx %v: %w",
			outpoint.Hash, err)
	}
	if int(outpoint.Index) >= len(closeTx.Vout) {
		return nil, fmt.Errorf("outpoint %v has invalid output index",
			outpoint)
	}

	vout := closeTx.Vout[outpoint.Index]
	if vout.Outspend != nil && vout.Outspend.Spent {
		return nil, fmt.Errorf("outpoint %v is already spent by %s:%d",
			outpoint, vout.Outspend.Txid, vout.Outspend.Vin)
	}

	pkScript, err := hex.DecodeString(vout.ScriptPubkey)
	if err != nil {
		return nil, fmt.Errorf("error decoding script for %v: %w",
			outpoint, err)
	}

	if len(closeTx.Vin) != 1 {
		return nil, fmt.Errorf("close tx %v has %d inputs, expected 1",
			outpoint.Hash, len(closeTx.Vin))
	}

	fundingHash, err := chainhash.NewHashFromStr(closeTx.Vin[0].Tixid)
	if err != nil {
		return nil, fmt.Errorf("error parsing funding txid: %w", err)
	}
	if closeTx.Vin[0].Vout < 0 {
		return nil, fmt.Errorf("close tx %v has negative funding vout",
			outpoint.Hash)
	}

	return &sweepHtlcTarget{
		outpoint: outpoint,
		fundingPoint: wire.OutPoint{
			Hash:  *fundingHash,
			Index: uint32(closeTx.Vin[0].Vout),
		},
		value:    int64(vout.Value),
		pkScript: pkScript,
		closeTx:  closeTx,
	}, nil
}

// parsePubKey parses a compressed or uncompressed public key hex string.
func parsePubKey(pubKeyHex string) (*btcec.PublicKey, error) {
	bytes, err := hex.DecodeString(strings.TrimSpace(pubKeyHex))
	if err != nil {
		return nil, err
	}

	return btcec.ParsePubKey(bytes)
}
