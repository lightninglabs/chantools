package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/lightningnetwork/lnd/aezeed"
	"golang.org/x/crypto/ssh/terminal"
)

type showRootKeyCommand struct{}

func (c *showRootKeyCommand) Execute(_ []string) error {
	// We'll now prompt the user to enter in their 24-word mnemonic.
	fmt.Printf("Input your 24-word mnemonic separated by spaces: ")
	reader := bufio.NewReader(os.Stdin)
	mnemonicStr, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	// We'll trim off extra spaces, and ensure the mnemonic is all
	// lower case, then populate our request.
	mnemonicStr = strings.TrimSpace(mnemonicStr)
	mnemonicStr = strings.ToLower(mnemonicStr)

	cipherSeedMnemonic := strings.Split(mnemonicStr, " ")

	fmt.Println()

	if len(cipherSeedMnemonic) != 24 {
		return fmt.Errorf("wrong cipher seed mnemonic "+
			"length: got %v words, expecting %v words",
			len(cipherSeedMnemonic), 24)
	}

	// Additionally, the user may have a passphrase, that will also
	// need to be provided so the daemon can properly decipher the
	// cipher seed.
	fmt.Printf("Input your cipher seed passphrase (press enter if " +
		"your seed doesn't have a passphrase): ")
	passphrase, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		return err
	}

	var mnemonic aezeed.Mnemonic
	copy(mnemonic[:], cipherSeedMnemonic[:])

	// If we're unable to map it back into the ciphertext, then either the
	// mnemonic is wrong, or the passphrase is wrong.
	cipherSeed, err := mnemonic.ToCipherSeed(passphrase)
	if err != nil {
		return err
	}
	rootKey, err := hdkeychain.NewMaster(cipherSeed.Entropy[:], chainParams)
	if err != nil {
		return fmt.Errorf("failed to derive master extended key")
	}
	fmt.Printf("\nYour BIP32 HD root key is: %s\n", rootKey.String())
	return nil
}
