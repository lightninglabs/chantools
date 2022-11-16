package main

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"
)

var (
	privKeyBytes, _ = hex.DecodeString(
		"571e2fc5e99f91596f7561da9f605cbf2e2342a166593eef041862b6a8b7" +
			"4f35",
	)
	pubKeyOrigBytes, _ = hex.DecodeString(
		"032ec305fb12642fd3b1091d1cba88ebb7b1a8dbc256b35789b7e223a1b3" +
			"75f0b7",
	)
	pubKeyNegBytes, _ = hex.DecodeString(
		"022ec305fb12642fd3b1091d1cba88ebb7b1a8dbc256b35789b7e223a1b3" +
			"75f0b7",
	)
	pubKeyNegTweakBytes, _ = hex.DecodeString(
		"0322b5c94ec4dc3a8843edc7448a0aad389d43e0f8d1b35b546dd1aad70f" +
			"b2c45b",
	)
	pubKeyNegTweakTweakBytes, _ = hex.DecodeString(
		"03f4cd1ff9efa8198e33e5a110dc690c1472d56c01287893c2f8ed55f61e" +
			"a767d1",
	)
)

func TestTweak(t *testing.T) {
	privKey, pubKey := btcec.PrivKeyFromBytes(privKeyBytes)
	require.Equal(t, pubKeyOrigBytes, pubKey.SerializeCompressed())

	privKeyCopy := copyPrivKey(privKey)
	require.Equal(t, privKey, privKeyCopy)

	mutateWithSign(privKeyCopy)
	require.NotEqual(t, privKey, privKeyCopy)
	require.Equalf(
		t, pubKeyNegBytes, privKeyCopy.PubKey().SerializeCompressed(),
		"%x", privKeyCopy.PubKey().SerializeCompressed(),
	)

	mutateWithTweak(privKeyCopy)
	require.NotEqual(t, privKey, privKeyCopy)
	require.Equalf(
		t, pubKeyNegTweakBytes,
		privKeyCopy.PubKey().SerializeCompressed(),
		"%x", privKeyCopy.PubKey().SerializeCompressed(),
	)

	mutateWithTweak(privKeyCopy)
	require.NotEqual(t, privKey, privKeyCopy)
	require.Equalf(
		t, pubKeyNegTweakTweakBytes,
		privKeyCopy.PubKey().SerializeCompressed(),
		"%x", privKeyCopy.PubKey().SerializeCompressed(),
	)
}
