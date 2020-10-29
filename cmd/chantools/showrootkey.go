package main

import (
	"fmt"

	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/btc"
	"github.com/guggero/chantools/lnd"
)

type showRootKeyCommand struct {
	BIP39 bool `long:"bip39" description:"Read a classic BIP39 seed and passphrase from the terminal instead of asking for the lnd seed format or providing the --rootkey flag."`
}

func (c *showRootKeyCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	var (
		extendedKey *hdkeychain.ExtendedKey
		err         error
	)

	// Check that root key is valid or fall back to terminal input.
	switch {
	case c.BIP39:
		extendedKey, err = btc.ReadMnemonicFromTerminal(chainParams)

	default:
		extendedKey, _, err = lnd.ReadAezeed(chainParams)
	}
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	fmt.Printf("\nYour BIP32 HD root key is: %s\n", extendedKey.String())
	return nil
}
