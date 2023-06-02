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

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightninglabs/chantools/bip39"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	BIP39MnemonicEnvName   = "SEED_MNEMONIC"
	BIP39PassphraseEnvName = "SEED_PASSPHRASE"
)

func ReadMnemonicFromTerminal(params *chaincfg.Params) (*hdkeychain.ExtendedKey,
	error) {

	var err error
	reader := bufio.NewReader(os.Stdin)

	// To automate things with chantools, we also offer reading the seed
	// from environment variables.
	mnemonicStr := strings.TrimSpace(os.Getenv(BIP39MnemonicEnvName))

	if mnemonicStr == "" {
		// If there's no value in the environment, we'll now prompt the
		// user to enter in their 12 to 24 word mnemonic.
		fmt.Printf("Input your 12 to 24 word mnemonic separated by " +
			"spaces: ")
		mnemonicStr, err = reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		fmt.Println()
	}

	// We'll trim off extra spaces, and ensure the mnemonic is all
	// lower case.
	mnemonicStr = strings.TrimSpace(mnemonicStr)
	mnemonicStr = strings.ToLower(mnemonicStr)

	mnemonicWords := strings.Split(mnemonicStr, " ")
	if len(mnemonicWords) < 12 || len(mnemonicWords) > 24 {
		return nil, errors.New("wrong cipher seed mnemonic length: " +
			"must be between 12 and 24 words")
	}

	// Additionally, the user may have a passphrase, that will also need to
	// be provided so the daemon can properly decipher the cipher seed.
	// Try the environment variable first.
	passphrase := strings.TrimSpace(os.Getenv(BIP39PassphraseEnvName))

	// Because we cannot differentiate between an empty and a non-existent
	// environment variable, we need a special character that indicates that
	// no passphrase should be used. We use a single dash (-) for that as
	// that would be too short for a passphrase anyway.
	var (
		passphraseBytes []byte
		seed            []byte
		choice          string
	)
	switch {
	// The user indicated in the environment variable that no passphrase
	// should be used. We don't set any value.
	case passphrase == "-":

	// The environment variable didn't contain anything, we'll read the
	// passphrase from the terminal.
	case passphrase == "":
		// Additionally, the user may have a passphrase, that will also
		// need to be provided so the daemon can properly decipher the
		// cipher seed.
		fmt.Printf("Input your cipher seed passphrase (press enter " +
			"if your seed doesn't have a passphrase): ")
		passphraseBytes, err = terminal.ReadPassword(
			int(syscall.Stdin), //nolint
		)
		if err != nil {
			return nil, err
		}
		fmt.Println()

		// Check that the mnemonic is valid.
		_, err = bip39.EntropyFromMnemonic(mnemonicStr)
		if err != nil {
			return nil, err
		}

		fmt.Printf("Please choose passphrase mode:\n" +
			"  0 - Default BIP39\n" +
			"  1 - Passphrase to hex\n" +
			"  2 - Digital Bitbox (extra round of PBKDF2)\n" +
			"\n" +
			"Choice [default 0]: ")
		choice, err = reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		fmt.Println()

	// There was a password in the environment, just convert it to bytes.
	default:
		passphraseBytes = []byte(passphrase)
	}

	switch strings.TrimSpace(choice) {
	case "", "0":
		seed = pbkdf2.Key(
			[]byte(mnemonicStr), append(
				[]byte("mnemonic"), passphraseBytes...,
			), 2048, 64, sha512.New,
		)

	case "1":
		p := []byte(hex.EncodeToString(passphraseBytes))
		seed = pbkdf2.Key(
			[]byte(mnemonicStr), append([]byte("mnemonic"), p...),
			2048, 64, sha512.New,
		)

	case "2":
		p := hex.EncodeToString(pbkdf2.Key(
			passphraseBytes, []byte("Digital Bitbox"), 20480, 64,
			sha512.New,
		))
		seed = pbkdf2.Key(
			[]byte(mnemonicStr), append([]byte("mnemonic"), p...),
			2048, 64, sha512.New,
		)

	default:
		return nil, fmt.Errorf("invalid mode selected: %v",
			choice)
	}

	rootKey, err := hdkeychain.NewMaster(seed, params)
	if err != nil {
		return nil, fmt.Errorf("failed to derive master extended "+
			"key: %w", err)
	}
	return rootKey, nil
}
