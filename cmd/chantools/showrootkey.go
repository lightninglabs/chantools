package main

import (
	"fmt"

	"github.com/guggero/chantools/lnd"
)

type showRootKeyCommand struct{}

func (c *showRootKeyCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	rootKey, _, err := lnd.ReadAezeedFromTerminal(chainParams)
	if err != nil {
		return fmt.Errorf("failed to read root key from console: %v",
			err)
	}
	fmt.Printf("\nYour BIP32 HD root key is: %s\n", rootKey.String())
	return nil
}
