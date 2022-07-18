package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const showRootKeyFormat = `
Your BIP32 HD root key is: %v
`

type showRootKeyCommand struct {
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

	cc.rootKey = newRootKey(cc.cmd, "decrypting the backup")

	return cc.cmd
}

func (c *showRootKeyCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	result := fmt.Sprintf(showRootKeyFormat, extendedKey)
	fmt.Println(result)

	// For the tests, also log as trace level which is disabled by default.
	log.Tracef(result)

	return nil
}
