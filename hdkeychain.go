package chantools

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/lightningnetwork/lnd/keychain"
)

const (
	hardenedKeyStart = uint32(hdkeychain.HardenedKeyStart)
)

func deriveChildren(key *hdkeychain.ExtendedKey, path []uint32) (
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

type channelBackupEncryptionRing struct {
	extendedKey *hdkeychain.ExtendedKey
	chainParams *chaincfg.Params
}

func (r *channelBackupEncryptionRing) DeriveNextKey(_ keychain.KeyFamily) (
	keychain.KeyDescriptor, error) {

	return keychain.KeyDescriptor{}, nil
}

func (r *channelBackupEncryptionRing) DeriveKey(keyLoc keychain.KeyLocator) (
	keychain.KeyDescriptor, error) {

	var empty = keychain.KeyDescriptor{}
	keyBackup, err := deriveChildren(r.extendedKey, []uint32{
		hardenedKeyStart + uint32(keychain.BIP0043Purpose),
		hardenedKeyStart + r.chainParams.HDCoinType,
		hardenedKeyStart + uint32(keyLoc.Family),
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
