package lnd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcwallet/snacl"
	"github.com/btcsuite/btcwallet/waddrmgr"
	"github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/walletdb"
	"github.com/lightningnetwork/lnd/aezeed"
	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/lightningnetwork/lnd/lnwallet"
	"go.etcd.io/bbolt"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	MnemonicEnvName   = "AEZEED_MNEMONIC"
	PassphraseEnvName = "AEZEED_PASSPHRASE"
	PasswordEnvName   = "WALLET_PASSWORD"
)

var (
	numberDotsRegex = regexp.MustCompile(`[\d.\-\n\r\t]*`)
	multipleSpaces  = regexp.MustCompile(" [ ]+")

	openCallbacks = &waddrmgr.OpenCallbacks{
		ObtainSeed:        noConsole,
		ObtainPrivatePass: noConsole,
	}

	// Namespace from github.com/btcsuite/btcwallet/wallet/wallet.go.
	WaddrmgrNamespaceKey = []byte("waddrmgr")

	// Bucket names from github.com/btcsuite/btcwallet/waddrmgr/db.go.
	mainBucketName    = []byte("main")
	masterPrivKeyName = []byte("mpriv")
	cryptoPrivKeyName = []byte("cpriv")
	masterHDPrivName  = []byte("mhdpriv")
)

func noConsole() ([]byte, error) {
	return nil, errors.New("wallet db requires console access")
}

// ReadAezeed reads an aezeed from the console or the environment variable.
func ReadAezeed(params *chaincfg.Params) (*hdkeychain.ExtendedKey, time.Time,
	error) {

	// To automate things with chantools, we also offer reading the seed
	// from environment variables.
	mnemonicStr := strings.TrimSpace(os.Getenv(MnemonicEnvName))

	// If nothing is set in the environment, read the seed from the
	// terminal.
	if mnemonicStr == "" {
		var err error
		// We'll now prompt the user to enter in their 24-word mnemonic.
		fmt.Printf("Input your 24-word mnemonic separated by spaces: ")
		reader := bufio.NewReader(os.Stdin)
		mnemonicStr, err = reader.ReadString('\n')
		if err != nil {
			return nil, time.Unix(0, 0), err
		}
	}

	// We'll trim off extra spaces, and ensure the mnemonic is all
	// lower case.
	mnemonicStr = strings.TrimSpace(mnemonicStr)
	mnemonicStr = strings.ToLower(mnemonicStr)

	// To allow the tool to also accept the copy/pasted version of the
	// backup text (which contains numbers and dots and multiple spaces),
	// we do some more cleanup with regex.
	mnemonicStr = numberDotsRegex.ReplaceAllString(mnemonicStr, "")
	mnemonicStr = multipleSpaces.ReplaceAllString(mnemonicStr, " ")
	mnemonicStr = strings.TrimSpace(mnemonicStr)

	cipherSeedMnemonic := strings.Split(mnemonicStr, " ")

	fmt.Println()

	if len(cipherSeedMnemonic) != 24 {
		return nil, time.Unix(0, 0), fmt.Errorf("wrong cipher seed "+
			"mnemonic length: got %v words, expecting %v words",
			len(cipherSeedMnemonic), 24)
	}

	passphraseBytes, err := ReadPassphrase("doesn't have")
	if err != nil {
		return nil, time.Unix(0, 0), err
	}

	var mnemonic aezeed.Mnemonic
	copy(mnemonic[:], cipherSeedMnemonic)

	// If we're unable to map it back into the ciphertext, then either the
	// mnemonic is wrong, or the passphrase is wrong.
	cipherSeed, err := mnemonic.ToCipherSeed(passphraseBytes)
	if err != nil {
		return nil, time.Unix(0, 0), fmt.Errorf("failed to decrypt "+
			"seed with passphrase: %w", err)
	}
	rootKey, err := hdkeychain.NewMaster(cipherSeed.Entropy[:], params)
	if err != nil {
		return nil, time.Unix(0, 0), errors.New("failed to derive " +
			"master extended key")
	}
	return rootKey, cipherSeed.BirthdayTime(), nil
}

// ReadPassphrase reads a cipher seed passphrase from the console or the
// environment variable.
func ReadPassphrase(verb string) ([]byte, error) {
	// Additionally, the user may have a passphrase, that will also need to
	// be provided so the daemon can properly decipher the cipher seed.
	// Try the environment variable first.
	passphrase := strings.TrimSpace(os.Getenv(PassphraseEnvName))

	// Because we cannot differentiate between an empty and a non-existent
	// environment variable, we need a special character that indicates that
	// no passphrase should be used. We use a single dash (-) for that as
	// that would be too short for a passphrase anyway.
	var passphraseBytes []byte
	switch {
	// The user indicated in the environment variable that no passphrase
	// should be used. We don't set any value.
	case passphrase == "-":

	// The environment variable didn't contain anything, we'll read the
	// passphrase from the terminal.
	case passphrase == "":
		var err error
		passphraseBytes, err = PasswordFromConsole(
			fmt.Sprintf("Input your cipher seed passphrase "+
				"(press enter if your seed %s a passphrase): ",
				verb),
		)
		if err != nil {
			return nil, err
		}

	// There was a password in the environment, just convert it to bytes.
	default:
		passphraseBytes = []byte(passphrase)
	}

	return passphraseBytes, nil
}

// PasswordFromConsole reads a password from the console or stdin.
func PasswordFromConsole(userQuery string) ([]byte, error) {
	fmt.Print(userQuery)

	// Read from terminal (if there is one).
	if terminal.IsTerminal(int(syscall.Stdin)) { //nolint
		pw, err := terminal.ReadPassword(int(syscall.Stdin)) //nolint
		if err != nil {
			return nil, err
		}

		fmt.Println()
		return pw, nil
	}

	// Read from stdin as a fallback.
	reader := bufio.NewReader(os.Stdin)
	pw, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	fmt.Println()
	return bytes.TrimSpace(pw), nil
}

// OpenWallet opens a lnd compatible wallet and returns it, along with the
// private wallet password.
func OpenWallet(walletDbPath string,
	chainParams *chaincfg.Params) (*wallet.Wallet, []byte, func() error,
	error) {

	var (
		publicWalletPw  = lnwallet.DefaultPublicPassphrase
		privateWalletPw = lnwallet.DefaultPrivatePassphrase
		err             error
	)

	// To automate things with chantools, we also offer reading the wallet
	// password from environment variables.
	pw := []byte(strings.TrimSpace(os.Getenv(PasswordEnvName)))

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
		pw, err = PasswordFromConsole("Input wallet password: ")
		if err != nil {
			return nil, nil, nil, err
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

	// Try to load and open the wallet.
	db, err := walletdb.Open(
		"bdb", lncfg.CleanAndExpandPath(walletDbPath), false,
		DefaultOpenTimeout, false,
	)
	if errors.Is(err, bbolt.ErrTimeout) {
		return nil, nil, nil, errors.New("error opening wallet " +
			"database, make sure lnd is not running and holding " +
			"the exclusive lock on the wallet")
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error opening wallet "+
			"database: %w", err)
	}

	w, err := wallet.Open(db, publicWalletPw, openCallbacks, chainParams, 0)
	if err != nil {
		_ = db.Close()
		return nil, nil, nil, fmt.Errorf("error opening wallet %w", err)
	}

	// Start and unlock the wallet.
	w.Start()
	err = w.Unlock(privateWalletPw, nil)
	if err != nil {
		w.Stop()
		_ = db.Close()
		return nil, nil, nil, err
	}

	cleanup := func() error {
		w.Stop()
		if err := db.Close(); err != nil {
			return err
		}

		return nil
	}

	return w, privateWalletPw, cleanup, nil
}

// DecryptWalletRootKey decrypts a lnd compatible wallet's root key.
func DecryptWalletRootKey(db walletdb.DB,
	privatePassphrase []byte) ([]byte, error) {

	// Step 1: Load the encryption parameters and encrypted keys from the
	// database.
	var masterKeyPrivParams []byte
	var cryptoKeyPrivEnc []byte
	var masterHDPrivEnc []byte
	err := walletdb.View(db, func(tx walletdb.ReadTx) error {
		ns := tx.ReadBucket(WaddrmgrNamespaceKey)
		if ns == nil {
			return fmt.Errorf("namespace '%s' does not exist",
				WaddrmgrNamespaceKey)
		}

		mainBucket := ns.NestedReadBucket(mainBucketName)
		if mainBucket == nil {
			return fmt.Errorf("bucket '%s' does not exist",
				mainBucketName)
		}

		val := mainBucket.Get(masterPrivKeyName)
		if val != nil {
			masterKeyPrivParams = make([]byte, len(val))
			copy(masterKeyPrivParams, val)
		}
		val = mainBucket.Get(cryptoPrivKeyName)
		if val != nil {
			cryptoKeyPrivEnc = make([]byte, len(val))
			copy(cryptoKeyPrivEnc, val)
		}
		val = mainBucket.Get(masterHDPrivName)
		if val != nil {
			masterHDPrivEnc = make([]byte, len(val))
			copy(masterHDPrivEnc, val)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Step 2: Unmarshal the master private key parameters and derive
	// key from passphrase.
	var masterKeyPriv snacl.SecretKey
	if err := masterKeyPriv.Unmarshal(masterKeyPrivParams); err != nil {
		return nil, err
	}
	if err := masterKeyPriv.DeriveKey(&privatePassphrase); err != nil {
		return nil, err
	}

	// Step 3: Decrypt the keys in the correct order.
	cryptoKeyPriv := &snacl.CryptoKey{}
	cryptoKeyPrivBytes, err := masterKeyPriv.Decrypt(cryptoKeyPrivEnc)
	if err != nil {
		return nil, err
	}
	copy(cryptoKeyPriv[:], cryptoKeyPrivBytes)
	return cryptoKeyPriv.Decrypt(masterHDPrivEnc)
}
