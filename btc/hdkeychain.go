package btc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/lightningnetwork/lnd/keychain"
)

const (
	HardenedKeyStart = uint32(hdkeychain.HardenedKeyStart)
)

func DeriveChildren(key *hdkeychain.ExtendedKey, path []uint32) (
	*hdkeychain.ExtendedKey, error) {

	var (
		currentKey = key
		err        error
	)
	for _, pathPart := range path {
		currentKey, err = currentKey.Child(pathPart)
		if err != nil {
			return nil, err
		}
	}
	return currentKey, nil
}

func ParsePath(path string) ([]uint32, error) {
	path = strings.TrimSpace(path)
	if len(path) == 0 {
		return nil, fmt.Errorf("path cannot be empty")
	}
	if !strings.HasPrefix(path, "m/") {
		return nil, fmt.Errorf("path must start with m/")
	}
	parts := strings.Split(path, "/")
	indices := make([]uint32, len(parts)-1)
	for i := 1; i < len(parts); i++ {
		index := uint32(0)
		part := parts[i]
		if strings.Contains(parts[i], "'") {
			index += HardenedKeyStart
			part = strings.TrimRight(parts[i], "'")
		}
		parsed, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("could not parse part \"%s\": "+
				"%v", part, err)
		}
		indices[i-1] = index + uint32(parsed)
	}
	return indices, nil
}

type HDKeyRing struct {
	ExtendedKey *hdkeychain.ExtendedKey
	ChainParams *chaincfg.Params
}

func (r *HDKeyRing) DeriveNextKey(_ keychain.KeyFamily) (
	keychain.KeyDescriptor, error) {

	return keychain.KeyDescriptor{}, nil
}

func (r *HDKeyRing) DeriveKey(keyLoc keychain.KeyLocator) (
	keychain.KeyDescriptor, error) {

	var empty = keychain.KeyDescriptor{}
	derivedKey, err := DeriveChildren(r.ExtendedKey, []uint32{
		HardenedKeyStart + uint32(keychain.BIP0043Purpose),
		HardenedKeyStart + r.ChainParams.HDCoinType,
		HardenedKeyStart + uint32(keyLoc.Family),
		0,
		keyLoc.Index,
	})
	if err != nil {
		return empty, err
	}

	derivedPubKey, err := derivedKey.ECPubKey()
	if err != nil {
		return empty, err
	}
	return keychain.KeyDescriptor{
		KeyLocator: keychain.KeyLocator{
			Family: keyLoc.Family,
			Index:  keyLoc.Index,
		},
		PubKey: derivedPubKey,
	}, nil
}

// Check if a key descriptor is correct by making sure that we can derive the
// key that it describes.
func (r *HDKeyRing) CheckDescriptor(
	keyDesc keychain.KeyDescriptor) error {

	// A check doesn't make sense if there is no public key set.
	if keyDesc.PubKey == nil {
		return fmt.Errorf("no public key provided to check")
	}

	// Performance fix, derive static path only once.
	familyKey, err := DeriveChildren(r.ExtendedKey, []uint32{
		HardenedKeyStart + uint32(keychain.BIP0043Purpose),
		HardenedKeyStart + r.ChainParams.HDCoinType,
		HardenedKeyStart + uint32(keyDesc.Family),
		0,
	})
	if err != nil {
		return err
	}

	// Scan the same key range as lnd would do on channel restore.
	for i := 0; i < keychain.MaxKeyRangeScan; i++ {
		child, err := DeriveChildren(familyKey, []uint32{uint32(i)})
		if err != nil {
			return err
		}
		pubKey, err := child.ECPubKey()
		if err != nil {
			return err
		}
		if !pubKey.IsEqual(keyDesc.PubKey) {
			continue
		}
		// If we found the key, we can abort and signal success.
		return nil
	}

	// We scanned the max range and didn't find a key. It's very likely not
	// derivable with the given information.
	return keychain.ErrCannotDerivePrivKey
}
