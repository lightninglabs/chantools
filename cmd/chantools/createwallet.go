package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	_ "github.com/btcsuite/btcwallet/walletdb/bdb"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/aezeed"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/lnwallet/btcwallet"
	"github.com/spf13/cobra"
)

type createWalletCommand struct {
	WalletDBDir  string
	GenerateSeed bool

	rootKey *rootKey
	cmd     *cobra.Command
}

func newCreateWalletCommand() *cobra.Command {
	cc := &createWalletCommand{}
	cc.cmd = &cobra.Command{
		Use: "createwallet",
		Short: "Create a new lnd compatible wallet.db file from an " +
			"existing seed or by generating a new one",
		Long: `Creates a new wallet that can be used with lnd or with 
chantools. The wallet can be created from an existing seed or a new one can be
generated (use --generateseed).`,
		Example: `chantools createwallet \
	--walletdbdir ~/.lnd/data/chain/bitcoin/mainnet`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.WalletDBDir, "walletdbdir", "", "the folder to create the "+
			"new wallet.db file in",
	)
	cc.cmd.Flags().BoolVar(
		&cc.GenerateSeed, "generateseed", false, "generate a new "+
			"seed instead of using an existing one",
	)

	cc.rootKey = newRootKey(cc.cmd, "creating the new wallet")

	return cc.cmd
}

func (c *createWalletCommand) Execute(_ *cobra.Command, _ []string) error {
	var (
		publicWalletPw  = lnwallet.DefaultPublicPassphrase
		privateWalletPw = lnwallet.DefaultPrivatePassphrase
		masterRootKey   *hdkeychain.ExtendedKey
		birthday        time.Time
		err             error
	)

	// Check that we have a wallet DB.
	if c.WalletDBDir == "" {
		return fmt.Errorf("wallet DB directory is required")
	}

	// Make sure the directory (and parents) exists.
	if err := os.MkdirAll(c.WalletDBDir, 0700); err != nil {
		return fmt.Errorf("error creating wallet DB directory '%s': %w",
			c.WalletDBDir, err)
	}

	// Check if we should create a new seed or read if from the console or
	// environment.
	if c.GenerateSeed {
		fmt.Printf("Generating new lnd compatible aezeed...\n")
		seed, err := aezeed.New(
			keychain.KeyDerivationVersionTaproot, nil, time.Now(),
		)
		if err != nil {
			return fmt.Errorf("error creating new seed: %w", err)
		}
		birthday = seed.BirthdayTime()

		// Derive the master extended key from the seed.
		masterRootKey, err = hdkeychain.NewMaster(
			seed.Entropy[:], chainParams,
		)
		if err != nil {
			return fmt.Errorf("failed to derive master extended "+
				"key: %w", err)
		}

		passphrase, err := lnd.ReadPassphrase("shouldn't use")
		if err != nil {
			return fmt.Errorf("error reading passphrase: %w", err)
		}

		mnemonic, err := seed.ToMnemonic(passphrase)
		if err != nil {
			return fmt.Errorf("error converting seed to "+
				"mnemonic: %w", err)
		}

		fmt.Println("Generated new seed")
		printCipherSeedWords(mnemonic[:])
	} else {
		masterRootKey, birthday, err = c.rootKey.readWithBirthday()
		if err != nil {
			return err
		}
	}

	// To automate things with chantools, we also offer reading the wallet
	// password from environment variables.
	pw := []byte(strings.TrimSpace(os.Getenv(lnd.PasswordEnvName)))

	// Because we cannot differentiate between an empty and a non-existent
	// environment variable, we need a special character that indicates that
	// no password should be used. We use a single dash (-) for that as that
	// would be too short for an explicit password anyway.
	switch {
	// The user indicated in the environment variable that no passphrase
	// should be used. We don't set any value.
	case string(pw) == "-":

	// The environment variable didn't contain anything, we'll read the
	// passphrase from the terminal.
	case len(pw) == 0:
		fmt.Printf("\n\nThe wallet password is used to encrypt the " +
			"wallet.db file itself and is unrelated to the seed.\n")
		pw, err = lnd.PasswordFromConsole("Input new wallet password: ")
		if err != nil {
			return err
		}
		pw2, err := lnd.PasswordFromConsole(
			"Confirm new wallet password: ",
		)
		if err != nil {
			return err
		}

		if !bytes.Equal(pw, pw2) {
			return fmt.Errorf("passwords don't match")
		}

		if len(pw) > 0 {
			publicWalletPw = pw
			privateWalletPw = pw
		}

	// There was a password in the environment, just use it directly.
	default:
		publicWalletPw = pw
		privateWalletPw = pw
	}

	// Try to create the wallet.
	loader, err := btcwallet.NewWalletLoader(
		chainParams, 0, btcwallet.LoaderWithLocalWalletDB(
			c.WalletDBDir, true, 0,
		),
	)
	if err != nil {
		return fmt.Errorf("error creating wallet loader: %w", err)
	}

	_, err = loader.CreateNewWalletExtendedKey(
		publicWalletPw, privateWalletPw, masterRootKey, birthday,
	)
	if err != nil {
		return fmt.Errorf("error creating new wallet: %w", err)
	}

	if err := loader.UnloadWallet(); err != nil {
		return fmt.Errorf("error unloading wallet: %w", err)
	}

	fmt.Printf("Wallet created successfully at %v\n", c.WalletDBDir)

	return nil
}

func printCipherSeedWords(mnemonicWords []string) {
	fmt.Println("!!!YOU MUST WRITE DOWN THIS SEED TO BE ABLE TO " +
		"RESTORE THE WALLET!!!")
	fmt.Println()

	fmt.Println("---------------BEGIN LND CIPHER SEED---------------")

	numCols := 4
	colWords := monoWidthColumns(mnemonicWords, numCols)
	for i := 0; i < len(colWords); i += numCols {
		fmt.Printf("%2d. %3s  %2d. %3s  %2d. %3s  %2d. %3s\n",
			i+1, colWords[i], i+2, colWords[i+1], i+3,
			colWords[i+2], i+4, colWords[i+3])
	}

	fmt.Println("---------------END LND CIPHER SEED-----------------")

	fmt.Println("\n!!!YOU MUST WRITE DOWN THIS SEED TO BE ABLE TO " +
		"RESTORE THE WALLET!!!")
}

// monoWidthColumns takes a set of words, and the number of desired columns,
// and returns a new set of words that have had white space appended to the
// word in order to create a mono-width column.
func monoWidthColumns(words []string, ncols int) []string {
	// Determine max size of words in each column.
	colWidths := make([]int, ncols)
	for i, word := range words {
		col := i % ncols
		curWidth := colWidths[col]
		if len(word) > curWidth {
			colWidths[col] = len(word)
		}
	}

	// Append whitespace to each word to make columns mono-width.
	finalWords := make([]string, len(words))
	for i, word := range words {
		col := i % ncols
		width := colWidths[col]

		diff := width - len(word)
		finalWords[i] = word + strings.Repeat(" ", diff)
	}

	return finalWords
}
