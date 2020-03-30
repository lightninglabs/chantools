package main

import (
	"fmt"

	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/lnd"
)

type deriveKeyCommand struct {
	RootKey string `long:"rootkey" description:"BIP32 HD root key to derive the key from."`
	Path    string `long:"path" description:"The BIP32 derivation path to derive. Must start with \"m/\"."`
	Neuter  bool   `long:"neuter" description:"Do not output the private key, just the public key."`
}

func (c *deriveKeyCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	var (
		extendedKey *hdkeychain.ExtendedKey
		err         error
	)

	// Check that root key is valid or fall back to console input.
	switch {
	case c.RootKey != "":
		extendedKey, err = hdkeychain.NewKeyFromString(c.RootKey)

	default:
		extendedKey, _, err = rootKeyFromConsole()
	}
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	return deriveKey(extendedKey, c.Path, c.Neuter)
}

func deriveKey(extendedKey *hdkeychain.ExtendedKey, path string,
	neuter bool) error {

	fmt.Printf("Deriving path %s for network %s.\n", path, chainParams.Name)
	pubKey, wif, err := lnd.DeriveKey(extendedKey, path, chainParams)
	if err != nil {
		return fmt.Errorf("could not derive keys: %v", err)
	}
	fmt.Printf("Public key: %x\n", pubKey.SerializeCompressed())

	if !neuter {
		fmt.Printf("Private key (WIF): %s\n", wif.String())
	}

	return nil
}
