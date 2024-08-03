package cln

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/keychain"
	"golang.org/x/crypto/hkdf"
)

const (
	KeyOffsetFunding    = 0
	KeyOffsetRevocation = 1
	KeyOffsetHtlc       = 2
	KeyOffsetPayment    = 3
	KeyOffsetDelayed    = 4
)

var (
	InfoNodeID     = []byte("nodeid")
	InfoPeerSeed   = []byte("peer seed")
	InfoPerPeer    = []byte("per-peer seed")
	InfoCLightning = []byte("c-lightning")
)

// NodeKey derives a CLN node key from the given HSM secret.
func NodeKey(hsmSecret [32]byte) (*btcec.PublicKey, *btcec.PrivateKey, error) {
	salt := make([]byte, 4)
	privKeyBytes, err := HkdfSha256(hsmSecret[:], salt, InfoNodeID)
	if err != nil {
		return nil, nil, err
	}

	privKey, pubKey := btcec.PrivKeyFromBytes(privKeyBytes[:])
	return pubKey, privKey, nil
}

// DeriveKeyPair derives a channel key pair from the given HSM secret, and the
// key descriptor. The public key in the key descriptor is used as the peer's
// public key, the family is converted to the CLN key type, and the index is
// used as the channel's database index.
func DeriveKeyPair(hsmSecret [32]byte,
	desc *keychain.KeyDescriptor) (*btcec.PublicKey, *btcec.PrivateKey,
	error) {

	var offset int
	switch desc.Family {
	case keychain.KeyFamilyMultiSig:
		offset = KeyOffsetFunding

	case keychain.KeyFamilyRevocationBase:
		offset = KeyOffsetRevocation

	case keychain.KeyFamilyHtlcBase:
		offset = KeyOffsetHtlc

	case keychain.KeyFamilyPaymentBase:
		offset = KeyOffsetPayment

	case keychain.KeyFamilyDelayBase:
		offset = KeyOffsetDelayed

	case keychain.KeyFamilyNodeKey:
		return NodeKey(hsmSecret)

	default:
		return nil, nil, fmt.Errorf("unsupported key family for CLN: "+
			"%v", desc.Family)
	}

	channelBase, err := HkdfSha256(hsmSecret[:], nil, InfoPeerSeed)
	if err != nil {
		return nil, nil, err
	}

	peerAndChannel := make([]byte, 33+8)
	copy(peerAndChannel[:33], desc.PubKey.SerializeCompressed())
	binary.LittleEndian.PutUint64(peerAndChannel[33:], uint64(desc.Index))

	channelSeed, err := HkdfSha256(
		channelBase[:], peerAndChannel, InfoPerPeer,
	)
	if err != nil {
		return nil, nil, err
	}

	fundingKey, err := HkdfSha256WithSkip(
		channelSeed[:], nil, InfoCLightning, offset*32,
	)
	if err != nil {
		return nil, nil, err
	}

	privKey, pubKey := btcec.PrivKeyFromBytes(fundingKey[:])
	return pubKey, privKey, nil
}

// HkdfSha256 derives a 32-byte key from the given input key material, salt, and
// info using the HKDF-SHA256 key derivation function.
func HkdfSha256(key, salt, info []byte) ([32]byte, error) {
	return HkdfSha256WithSkip(key, salt, info, 0)
}

// HkdfSha256WithSkip derives a 32-byte key from the given input key material,
// salt, and info using the HKDF-SHA256 key derivation function and skips the
// first `skip` bytes of the output.
func HkdfSha256WithSkip(key, salt, info []byte, skip int) ([32]byte, error) {
	expander := hkdf.New(sha256.New, key, salt, info)

	if skip > 0 {
		skippedBytes := make([]byte, skip)
		_, err := expander.Read(skippedBytes)
		if err != nil {
			return [32]byte{}, err
		}
	}

	var outputKey [32]byte
	_, err := expander.Read(outputKey[:])
	if err != nil {
		return [32]byte{}, err
	}

	return outputKey, nil
}
