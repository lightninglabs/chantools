package main

import (
	"fmt"

	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/btc"
)

type deriveKeyCommand struct {
	RootKey string `long:"rootkey" description:"BIP32 HD root key to derive the key from."`
	Path    string `long:"path" description:"The BIP32 derivation path to derive. Must start with \"m/\"."`
	Neuter  bool   `long:"neuter" description:"Do not output the private key, just the public key."`
}

func (c *deriveKeyCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	// Check that root key is valid.
	if c.RootKey == "" {
		return fmt.Errorf("root key is required")
	}
	extendedKey, err := hdkeychain.NewKeyFromString(c.RootKey)
	if err != nil {
		return fmt.Errorf("error parsing root key: %v", err)
	}

	return deriveKey(extendedKey, c.Path, c.Neuter)
}

func deriveKey(extendedKey *hdkeychain.ExtendedKey, path string,
	neuter bool) error {

	fmt.Printf("Deriving path %s for network %s.\n", path, chainParams.Name)
	parsedPath, err := btc.ParsePath(path)
	if err != nil {
		return fmt.Errorf("could not parse derivation path: %v", err)
	}
	derivedKey, err := btc.DeriveChildren(extendedKey, parsedPath)
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
