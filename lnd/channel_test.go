package lnd

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

var (
	rootKey = "tprv8ZgxMBicQKsPejNXQLJKe3dBBs9Zrt53EZrsBzVLQ8rZji3" +
		"hVb3wcoRvgrjvTmjPG2ixoGUUkCyC6yBEy9T5gbLdvD2a5VmJbcFd5Q9pkAs"

	staticRand = [32]byte{
		0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8,
		0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8,
		0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8,
		0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8,
	}

	staticChanPoint = &wire.OutPoint{
		Hash: chainhash.Hash{
			0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8,
		},
		Index: 123,
	}

	testNetParams = &chaincfg.TestNet3Params
	mainNetParams = &chaincfg.MainNetParams

	staticPubNonceHex = "0275757be33335347132895c3cf7c9d5d4c6dbfbc2b8090b" +
		"0c311929b5b3304629026f5183811ea44bd60110f9d4c30525bb1e8c72f9" +
		"19b766464e91db7739d4123a"
)

func TestGenerateMuSig2Nonces(t *testing.T) {
	extendedKey, err := hdkeychain.NewKeyFromString(rootKey)
	require.NoError(t, err)

	staticNonces, err := GenerateMuSig2Nonces(
		extendedKey, staticRand, staticChanPoint, testNetParams, nil,
	)
	require.NoError(t, err)

	require.Equal(
		t, staticPubNonceHex,
		hex.EncodeToString(staticNonces.PubNonce[:]),
	)

	testCases := []struct {
		name        string
		randomness  [32]byte
		chanPoint   *wire.OutPoint
		chainParams *chaincfg.Params
		pubNonce    string
	}{{
		name:        "mainnet",
		randomness:  staticRand,
		chanPoint:   staticChanPoint,
		chainParams: mainNetParams,
		pubNonce: "02045795da7cffa1e2d8e64c4dfe606cd54b9d727e93f8c277" +
			"5b1d5442b80f605c024471a42dae0583f08262dcd09162d692bd" +
			"2ceb44178f37599c925b6465e92786",
	}, {
		name:       "channel point",
		randomness: staticRand,
		chanPoint: &wire.OutPoint{
			Hash: chainhash.Hash{
				0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8,
			},
			Index: 124,
		},
		chainParams: testNetParams,
		pubNonce: "025c22d8bc5fd0605fa007db40977da4caff2d5312bd865b62" +
			"b7db6a184ea1b0d803278833480a3b7005cd1fad18c9a2740407" +
			"8a325d3c85f33b0370663d2943e44d",
	}, {
		name:        "randomness",
		randomness:  [32]byte{0x1},
		chanPoint:   staticChanPoint,
		chainParams: testNetParams,
		pubNonce: "02a0d0b3281e92130e64a454ad122b37c8fd771647eb442769" +
			"103b583db5d73753030ed0c6e04f4fb729b2db5a34331a4e5283" +
			"b3872004222c401a4b9ec6d0540f64",
	}}

	for idx := range testCases {
		tc := testCases[idx]

		t.Run(tc.name, func(t *testing.T) {
			nonces, err := GenerateMuSig2Nonces(
				extendedKey, tc.randomness, tc.chanPoint,
				tc.chainParams, nil,
			)
			require.NoError(t, err)

			require.NotEqual(
				t, staticNonces.PubNonce, nonces.PubNonce,
			)

			require.Equal(
				t, tc.pubNonce,
				hex.EncodeToString(nonces.PubNonce[:]),
			)
		})
	}
}
