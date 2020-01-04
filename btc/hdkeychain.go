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

type ChannelBackupEncryptionRing struct {
	ExtendedKey *hdkeychain.ExtendedKey
	ChainParams *chaincfg.Params
}

func (r *ChannelBackupEncryptionRing) DeriveNextKey(_ keychain.KeyFamily) (
	keychain.KeyDescriptor, error) {

	return keychain.KeyDescriptor{}, nil
}

func (r *ChannelBackupEncryptionRing) DeriveKey(keyLoc keychain.KeyLocator) (
	keychain.KeyDescriptor, error) {

	var empty = keychain.KeyDescriptor{}
	keyBackup, err := DeriveChildren(r.ExtendedKey, []uint32{
		HardenedKeyStart + uint32(keychain.BIP0043Purpose),
		HardenedKeyStart + r.ChainParams.HDCoinType,
		HardenedKeyStart + uint32(keyLoc.Family),
		0,
		keyLoc.Index,
	})
	if err != nil {
		return empty, err
	}

	backupPubKey, err := keyBackup.ECPubKey()
	if err != nil {
		return empty, err
	}
	return keychain.KeyDescriptor{
		KeyLocator: keychain.KeyLocator{
			Family: keyLoc.Family,
			Index:  keyLoc.Index,
		},
		PubKey: backupPubKey,
	}, nil
}
