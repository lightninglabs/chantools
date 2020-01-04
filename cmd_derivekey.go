package chantools

import (
	"fmt"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"
)

func deriveKey(extendedKey *hdkeychain.ExtendedKey, path string,
	neuter bool) error {

	fmt.Printf("Deriving path %s for network %s.\n", path, chainParams.Name)
	parsedPath, err := parsePath(path)
	if err != nil {
		return fmt.Errorf("could not parse derivation path: %v", err)
	}
	derivedKey, err := deriveChildren(extendedKey, parsedPath)
	if err != nil {
		return fmt.Errorf("could not derive children: %v", err)
	}
	pubKey, err := derivedKey.ECPubKey()
	if err != nil {
		return fmt.Errorf("could not derive public key: %v", err)
	}
	fmt.Printf("Public key: %x\n", pubKey.SerializeCompressed())

	if neuter {
		return nil
	}

	privKey, err := derivedKey.ECPrivKey()
	if err != nil {
		return fmt.Errorf("could not derive private key: %v", err)
	}
	wif, err := btcutil.NewWIF(privKey, chainParams, true)
	if err != nil {
		return fmt.Errorf("could not encode WIF: %v", err)
	}
	fmt.Printf("Private key (WIF): %s\n", wif.String())

	return nil
}
