package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcwallet/snacl"
	"github.com/btcsuite/btcwallet/waddrmgr"
	"github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/walletdb"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwallet"

	// This is required to register bdb as a valid walletdb driver. In the
	// init function of the package, it registers itself. The import is used
	// to activate the side effects w/o actually binding the package name to
	// a file-level variable.
	_ "github.com/btcsuite/btcwallet/walletdb/bdb"
)

var (
	// Namespace from github.com/btcsuite/btcwallet/wallet/wallet.go
	waddrmgrNamespaceKey = []byte("waddrmgr")

	// Bucket names from github.com/btcsuite/btcwallet/waddrmgr/db.go
	mainBucketName    = []byte("main")
	masterPrivKeyName = []byte("mpriv")
	cryptoPrivKeyName = []byte("cpriv")
	masterHDPrivName  = []byte("mhdpriv")
	defaultAccount    = uint32(waddrmgr.DefaultAccountNum)
	openCallbacks     = &waddrmgr.OpenCallbacks{
		ObtainSeed:        noConsole,
		ObtainPrivatePass: noConsole,
	}
)

type walletInfoCommand struct {
	WalletDB    string `long:"walletdb" description:"The lnd wallet.db file to dump the contents from."`
	WithRootKey bool   `long:"withrootkey" description:"Should the BIP32 HD root key of the wallet be printed to standard out?"`
}

func (c *walletInfoCommand) Execute(_ []string) error {
	var (
		publicWalletPw  = lnwallet.DefaultPublicPassphrase
		privateWalletPw = lnwallet.DefaultPrivatePassphrase
	)

	// Check that we have a wallet DB.
	if c.WalletDB == "" {
		return fmt.Errorf("wallet DB is required")
	}

	// Ask the user for the wallet password. If it's empty, the default
	// password will be used, since the lnd wallet is always encrypted.
	pw, err := passwordFromConsole("Input wallet password: ")
	if err != nil {
		return err
	}
	if len(pw) > 0 {
		publicWalletPw = pw
		privateWalletPw = pw
	}

	// Try to load and open the wallet.
	db, err := walletdb.Open("bdb", cleanAndExpandPath(c.WalletDB), false)
	if err != nil {
		return fmt.Errorf("error opening wallet database: %v", err)
	}
	defer closeWalletDb(db)
	w, err := wallet.Open(db, publicWalletPw, openCallbacks, chainParams, 0)
	if err != nil {
		return err
	}

	// Start and unlock the wallet.
	w.Start()
	defer w.Stop()
	err = w.Unlock(privateWalletPw, nil)
	if err != nil {
		return err
	}

	// Print the wallet info and if requested the root key.
	err = walletInfo(w)
	if err != nil {
		return err
	}
	if c.WithRootKey {
		masterHDPrivKey, err := decryptRootKey(db, privateWalletPw)
		if err != nil {
			return err
		}
		fmt.Printf("BIP32 HD extended root key: %s\n", masterHDPrivKey)
	}
	return nil
}

func walletInfo(w *wallet.Wallet) error {
	keyRing := keychain.NewBtcWalletKeyRing(w, chainParams.HDCoinType)
	idPrivKey, err := keyRing.DerivePrivKey(keychain.KeyDescriptor{
		KeyLocator: keychain.KeyLocator{
			Family: keychain.KeyFamilyNodeKey,
			Index:  0,
		},
	})
	if err != nil {
		return err
	}
	idPrivKey.Curve = btcec.S256()
	fmt.Printf(
		"Identity Pubkey: %s\n",
		hex.EncodeToString(idPrivKey.PubKey().SerializeCompressed()),
	)

	// Print information about the different addresses in use.
	printScopeInfo(
		"np2wkh", w,
		w.Manager.ScopesForExternalAddrType(
			waddrmgr.NestedWitnessPubKey,
		),
	)
	printScopeInfo(
		"p2wkh", w,
		w.Manager.ScopesForExternalAddrType(
			waddrmgr.WitnessPubKey,
		),
	)
	return nil
}

func printScopeInfo(name string, w *wallet.Wallet, scopes []waddrmgr.KeyScope) {
	for _, scope := range scopes {
		props, err := w.AccountProperties(scope, defaultAccount)
		if err != nil {
			fmt.Printf("Error fetching account properties: %v", err)
		}
		fmt.Printf("Scope: %s\n", scope.String())
		fmt.Printf(
			"  Number of internal (change) %s addresses: %d\n",
			name, props.InternalKeyCount,
		)
		fmt.Printf(
			"  Number of external %s addresses: %d\n", name,
			props.ExternalKeyCount,
		)
	}
}

func decryptRootKey(db walletdb.DB, privPassphrase []byte) ([]byte, error) {
	// Step 1: Load the encryption parameters and encrypted keys from the
	// database.
	var masterKeyPrivParams []byte
	var cryptoKeyPrivEnc []byte
	var masterHDPrivEnc []byte
	err := walletdb.View(db, func(tx walletdb.ReadTx) error {
		ns := tx.ReadBucket(waddrmgrNamespaceKey)
		if ns == nil {
			return fmt.Errorf(
				"namespace '%s' does not exist",
				waddrmgrNamespaceKey,
			)
		}

		mainBucket := ns.NestedReadBucket(mainBucketName)
		if mainBucket == nil {
			return fmt.Errorf(
				"bucket '%s' does not exist",
				mainBucketName,
			)
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
	if err := masterKeyPriv.DeriveKey(&privPassphrase); err != nil {
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

func closeWalletDb(db walletdb.DB) {
	err := db.Close()
	if err != nil {
		fmt.Printf("Error closing database: %v", err)
	}
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
// This function is taken from https://github.com/btcsuite/btcd
func cleanAndExpandPath(path string) string {
	if path == "" {
		return ""
	}

	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		var homeDir string
		u, err := user.Current()
		if err == nil {
			homeDir = u.HomeDir
		} else {
			homeDir = os.Getenv("HOME")
		}

		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but the variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}
