package main

import (
	"fmt"
	"github.com/btcsuite/btcutil"
	"github.com/guggero/chantools/btc"

	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/lnd"
)

type deriveKeyCommand struct {
	BIP39   bool   `long:"bip39" description:"Read a classic BIP39 seed and passphrase from the terminal instead of asking for the lnd seed format or providing the --rootkey flag."`
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

	// Check that root key is valid or fall back to terminal input.
	switch {
	case c.BIP39:
		extendedKey, err = btc.ReadMnemonicFromTerminal(chainParams)

	case c.RootKey != "":
		extendedKey, err = hdkeychain.NewKeyFromString(c.RootKey)

	default:
		extendedKey, _, err = lnd.ReadAezeed(chainParams)
	}
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	return deriveKey(extendedKey, c.Path, c.Neuter)
}

func deriveKey(extendedKey *hdkeychain.ExtendedKey, path string,
	neuter bool) error {

	fmt.Printf("Deriving path %s for network %s.\n", path, chainParams.Name)
	child, pubKey, wif, err := lnd.DeriveKey(extendedKey, path, chainParams)
	if err != nil {
		return fmt.Errorf("could not derive keys: %v", err)
	}
	neutered, err := child.Neuter()
	if err != nil {
		return fmt.Errorf("could not neuter child key: %v", err)
	}
	fmt.Printf("\nPublic key: %x\n", pubKey.SerializeCompressed())
	fmt.Printf("Extended public key (xpub): %s\n", neutered.String())

	// Print the address too.
	hash160 := btcutil.Hash160(pubKey.SerializeCompressed())
	addrP2PKH, err := btcutil.NewAddressPubKeyHash(hash160, chainParams)
	if err != nil {
		return fmt.Errorf("could not create address: %v", err)
	}
	addrP2WKH, err := btcutil.NewAddressWitnessPubKeyHash(
		hash160, chainParams,
	)
	if err != nil {
		return fmt.Errorf("could not create address: %v", err)
	}
	fmt.Printf("Address: %s\n", addrP2WKH)
	fmt.Printf("Legacy address: %s\n", addrP2PKH)

	if !neuter {
		fmt.Printf("\nPrivate key (WIF): %s\n", wif.String())
		fmt.Printf("Extended private key (xprv): %s\n", child.String())
	}

	return nil
}
