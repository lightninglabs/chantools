package main

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/require"
)

var (
	key1Bytes, _ = hex.DecodeString(
		"0201943d78d61c8ad50ba57164830f536c156d8d89d979448bef3e67f564" +
			"ea0ab6",
	)
	key1, _      = btcec.ParsePubKey(key1Bytes)
	key2Bytes, _ = hex.DecodeString(
		"038b88de18064024e9da4dfc9c804283b3077a265dcd73ad3615b50badcb" +
			"debd5b",
	)
	key2, _ = btcec.ParsePubKey(key2Bytes)
	addr    = "bc1qp5jnhnavt32fjwhnf5ttpvvym7e0syp79q5l9skz545q62d8u2uq05" +
		"ul63"
)

func TestMatchScript(t *testing.T) {
	ok, _, _, err := matchScript(addr, key1, key2, &chaincfg.MainNetParams)
	require.NoError(t, err)
	require.True(t, ok)
}
