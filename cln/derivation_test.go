package cln

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/stretchr/testify/require"
)

var (
	hsmSecret = [32]byte{
		0x3f, 0x0a, 0x06, 0xc6, 0x38, 0x5b, 0x74, 0x93,
		0xf7, 0x5a, 0xa0, 0x08, 0x9f, 0x31, 0x6a, 0x13,
		0xbf, 0x72, 0xbe, 0xb4, 0x30, 0xe5, 0x9e, 0x71,
		0xb5, 0xac, 0x5a, 0x73, 0x58, 0x1a, 0x62, 0x70,
	}
	nodeKeyBytes, _ = hex.DecodeString(
		"035149629152c1bee83f1e148a51400b5f24bf3e2ca53384dd801418446e" +
			"1f53fe",
	)

	peerPubKeyBytes, _ = hex.DecodeString(
		"02678187ca43e6a6f62f9185be98a933bf485313061e6a05578bbd83c54e" +
			"88d460",
	)
	peerPubKey, _ = btcec.ParsePubKey(peerPubKeyBytes)

	expectedFundingKeyBytes, _ = hex.DecodeString(
		"0326a2171c97673cc8cd7a04a043f0224c59591fc8c9de320a48f7c9b68a" +
			"b0ae2b",
	)
)

func TestNodeKey(t *testing.T) {
	nodeKey, _, err := NodeKey(hsmSecret)
	require.NoError(t, err)

	require.Equal(t, nodeKeyBytes, nodeKey.SerializeCompressed())
}

func TestFundingKey(t *testing.T) {
	fundingKey, _, err := DeriveKeyPair(hsmSecret, &keychain.KeyDescriptor{
		PubKey: peerPubKey,
		KeyLocator: keychain.KeyLocator{
			Family: keychain.KeyFamilyMultiSig,
			Index:  1,
		},
	})
	require.NoError(t, err)

	require.Equal(
		t, expectedFundingKeyBytes, fundingKey.SerializeCompressed(),
	)
}

func TestPaymentBasePointSecret(t *testing.T) {
	hsmSecret2, _ := hex.DecodeString(
		"665b09e6fc86391f0141d957eb14ec30f8f8a58a876842792474cacc2448" +
			"9456",
	)

	basePointPeerBytes, _ := hex.DecodeString(
		"0350aeef9f33a157953d3c3c1ef464bdf421204461959524e52e530c17f1" +
			"66f541",
	)

	expectedPaymentBasePointBytes, _ := hex.DecodeString(
		"0339c93ca896829672510f8a4e51caef4b5f6a26f880acf5a120725a7f02" +
			"7b56b4",
	)

	var hsmSecret [32]byte
	copy(hsmSecret[:], hsmSecret2)

	basepointPeer, err := btcec.ParsePubKey(basePointPeerBytes)
	require.NoError(t, err)

	nk, _, err := NodeKey(hsmSecret)
	require.NoError(t, err)

	t.Logf("Node key: %x", nk.SerializeCompressed())

	fk, _, err := DeriveKeyPair(hsmSecret, &keychain.KeyDescriptor{
		PubKey: basepointPeer,
		KeyLocator: keychain.KeyLocator{
			Family: keychain.KeyFamilyMultiSig,
			Index:  1,
		},
	})
	require.NoError(t, err)

	t.Logf("Funding key: %x", fk.SerializeCompressed())

	paymentBasePoint, _, err := DeriveKeyPair(
		hsmSecret, &keychain.KeyDescriptor{
			PubKey: basepointPeer,
			KeyLocator: keychain.KeyLocator{
				Family: keychain.KeyFamilyPaymentBase,
				Index:  1,
			},
		},
	)
	require.NoError(t, err)

	require.Equal(
		t, expectedPaymentBasePointBytes,
		paymentBasePoint.SerializeCompressed(),
	)
}
