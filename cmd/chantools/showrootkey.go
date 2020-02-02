package main

import (
	"fmt"
)

type showRootKeyCommand struct{}

func (c *showRootKeyCommand) Execute(_ []string) error {
	rootKey, _, err := rootKeyFromConsole()
	if err != nil {
		return fmt.Errorf("failed to read root key from console: %v",
			err)
	}
	fmt.Printf("\nYour BIP32 HD root key is: %s\n", rootKey.String())
	return nil
}
