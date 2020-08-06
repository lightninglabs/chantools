package btc

import (
	"bufio"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/bip39"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/ssh/terminal"
)

func ReadMnemonicFromTerminal(params *chaincfg.Params) (*hdkeychain.ExtendedKey,
	error) {

	// We'll now prompt the user to enter in their 12 to 24 word mnemonic.
	fmt.Printf("Input your 12 to 24 word mnemonic separated by spaces: ")
	reader := bufio.NewReader(os.Stdin)
	mnemonicStr, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	fmt.Println()

	// We'll trim off extra spaces, and ensure the mnemonic is all
	// lower case.
	mnemonicStr = strings.TrimSpace(mnemonicStr)
	mnemonicStr = strings.ToLower(mnemonicStr)

	mnemonicWords := strings.Split(mnemonicStr, " ")
	if len(mnemonicWords) < 12 || len(mnemonicWords) > 24 {
		return nil, errors.New("wrong cipher seed mnemonic length: " +
			"must be between 12 and 24 words")
	}

	// Additionally, the user may have a passphrase, that will also
	// need to be provided so the daemon can properly decipher the
	// cipher seed.
	fmt.Printf("Input your cipher seed passphrase (press enter if " +
		"your seed doesn't have a passphrase): ")
	passphrase, err := terminal.ReadPassword(int(syscall.Stdin)) // nolint
	if err != nil {
		return nil, err
	}
	fmt.Println()

	// Check that the mnemonic is valid.
	_, err = bip39.EntropyFromMnemonic(mnemonicStr)
	if err != nil {
		return nil, err
	}

	var seed []byte
	fmt.Printf("Please choose passphrase mode:\n" +
		"  0 - Default BIP39\n" +
		"  1 - Passphrase to hex\n" +
		"  2 - Digital Bitbox (extra round of PBKDF2)\n" +
		"\n" +
		"Choice [default 0]: ")
	choice, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	fmt.Println()

	switch strings.TrimSpace(choice) {
	case "", "0":
		seed = pbkdf2.Key(
			[]byte(mnemonicStr), append(
				[]byte("mnemonic"), passphrase...,
			), 2048, 64, sha512.New,
		)

	case "1":
		passphrase = []byte(hex.EncodeToString(passphrase))
		seed = pbkdf2.Key(
			[]byte(mnemonicStr), append(
				[]byte("mnemonic"), passphrase...,
			), 2048, 64, sha512.New,
		)

	case "2":
		passphrase = pbkdf2.Key(
			passphrase, []byte("Digital Bitbox"), 20480, 64,
			sha512.New,
		)
		passphrase = []byte(hex.EncodeToString(passphrase))
		seed = pbkdf2.Key(
			[]byte(mnemonicStr), append(
				[]byte("mnemonic"), passphrase...,
			), 2048, 64, sha512.New,
		)

	default:
		return nil, fmt.Errorf("invalid mode selected: %v",
			choice)
	}

	rootKey, err := hdkeychain.NewMaster(seed, params)
	if err != nil {
		return nil, fmt.Errorf("failed to derive master extended "+
			"key: %v", err)
	}
	return rootKey, nil
}
