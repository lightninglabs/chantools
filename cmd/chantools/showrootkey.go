package main

import (
	"fmt"

	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/btc"
	"github.com/spf13/cobra"
)

type showRootKeyCommand struct {
	BIP39 bool

	rootKey *rootKey
	cmd     *cobra.Command
}

func newShowRootKeyCommand() *cobra.Command {
	cc := &showRootKeyCommand{}
	cc.cmd = &cobra.Command{
		Use: "showrootkey",
		Short: "Extract and show the BIP32 HD root key from the 24 " +
			"word lnd aezeed",
		Long: `This command converts the 24 word lnd aezeed phrase and
password to the BIP32 HD root key that is used as the --rootkey flag in other
commands of this tool.`,
		Example: `chantools showrootkey`,
		RunE:    cc.Execute,
	}
	cc.cmd.Flags().BoolVar(
		&cc.BIP39, "bip39", false, "read a classic BIP39 seed and "+
			"passphrase from the terminal instead of asking for "+
			"lnd seed format or providing the --rootkey flag",
	)

	cc.rootKey = newRootKey(cc.cmd, "decrypting the backup")

	return cc.cmd
}

func (c *showRootKeyCommand) Execute(_ *cobra.Command, _ []string) error {
	var (
		extendedKey *hdkeychain.ExtendedKey
		err         error
	)

	// Check that root key is valid or fall back to terminal input.
	switch {
	case c.BIP39:
		extendedKey, err = btc.ReadMnemonicFromTerminal(chainParams)

	default:
		extendedKey, err = c.rootKey.read()
	}
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	fmt.Printf("\nYour BIP32 HD root key is: %s\n", extendedKey.String())
	return nil
}
