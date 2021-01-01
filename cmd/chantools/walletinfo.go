package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcwallet/snacl"
	"github.com/btcsuite/btcwallet/waddrmgr"
	"github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/walletdb"
	"github.com/guggero/chantools/lnd"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/spf13/cobra"

	// This is required to register bdb as a valid walletdb driver. In the
	// init function of the package, it registers itself. The import is used
	// to activate the side effects w/o actually binding the package name to
	// a file-level variable.
	_ "github.com/btcsuite/btcwallet/walletdb/bdb"
)

const (
	passwordEnvName = "WALLET_PASSWORD"

	walletInfoFormat = `
Identity Pubkey:		%x
BIP32 HD extended root key:	%s
Wallet scopes:
%s
`

	keyScopeformat = `
Scope:	m/%d'/%d'
  Number of internal %s addresses:	%d
  Number of external %s addresses: 	%d
`
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
	WalletDB    string
	WithRootKey bool

	cmd *cobra.Command
}

func newWalletInfoCommand() *cobra.Command {
	cc := &walletInfoCommand{}
	cc.cmd = &cobra.Command{
		Use: "walletinfo",
		Short: "Shows info about an lnd wallet.db file and optionally " +
			"extracts the BIP32 HD root key",
		Long: `Shows some basic information about an lnd wallet.db file,
like the node identity the wallet belongs to, how many on-chain addresses are
used and, if enabled with --withrootkey the BIP32 HD root key of the wallet. The
latter can be useful to recover funds from a wallet if the wallet password is
still known but the seed was lost. **The 24 word seed phrase itself cannot be
extracted** because it is hashed into the extended HD root key before storing it
in the wallet.db.`,
		Example: `chantools walletinfo --withrootkey \
	--walletdb ~/.lnd/data/chain/bitcoin/mainnet/wallet.db`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.WalletDB, "walletdb", "", "lnd wallet.db file to dump the "+
			"contents from",
	)
	cc.cmd.Flags().BoolVar(
		&cc.WithRootKey, "withrootkey", false, "print BIP32 HD root "+
			"key of wallet to standard out",
	)

	return cc.cmd
}

func (c *walletInfoCommand) Execute(_ *cobra.Command, _ []string) error {
	var (
		publicWalletPw  = lnwallet.DefaultPublicPassphrase
		privateWalletPw = lnwallet.DefaultPrivatePassphrase
		err             error
	)

	// Check that we have a wallet DB.
	if c.WalletDB == "" {
		return fmt.Errorf("wallet DB is required")
	}

	// To automate things with chantools, we also offer reading the wallet
	// password from environment variables.
	pw := []byte(strings.TrimSpace(os.Getenv(passwordEnvName)))

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
		pw, err = passwordFromConsole("Input wallet password: ")
		if err != nil {
			return err
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
		"bdb", lncfg.CleanAndExpandPath(c.WalletDB), false,
		lnd.DefaultOpenTimeout,
	)
	if err != nil {
		return fmt.Errorf("error opening wallet database: %v", err)
	}
	defer func() { _ = db.Close() }()

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
	identityKey, scopeInfo, err := walletInfo(w)
	if err != nil {
		return err
	}
	rootKey := "n/a"
	if c.WithRootKey {
		masterHDPrivKey, err := decryptRootKey(db, privateWalletPw)
		if err != nil {
			return err
		}
		rootKey = string(masterHDPrivKey)
	}

	result := fmt.Sprintf(
		walletInfoFormat, identityKey.SerializeCompressed(), rootKey,
		scopeInfo,
	)

	fmt.Printf(result)

	// For the tests, also log as trace level which is disabled by default.
	log.Tracef(result)

	return nil
}

func walletInfo(w *wallet.Wallet) (*btcec.PublicKey, string, error) {
	keyRing := keychain.NewBtcWalletKeyRing(w, chainParams.HDCoinType)
	idPrivKey, err := keyRing.DerivePrivKey(keychain.KeyDescriptor{
		KeyLocator: keychain.KeyLocator{
			Family: keychain.KeyFamilyNodeKey,
			Index:  0,
		},
	})
	if err != nil {
		return nil, "", fmt.Errorf("unable to open key ring for coin "+
			"type %d: %v", chainParams.HDCoinType, err)
	}

	// Collect information about the different addresses in use.
	scopeNp2wkh, err := printScopeInfo(
		"np2wkh", w, w.Manager.ScopesForExternalAddrType(
			waddrmgr.NestedWitnessPubKey,
		),
	)
	if err != nil {
		return nil, "", err
	}
	scopeP2wkh, err := printScopeInfo(
		"p2wkh", w, w.Manager.ScopesForExternalAddrType(
			waddrmgr.WitnessPubKey,
		),
	)
	if err != nil {
		return nil, "", err
	}

	return idPrivKey.PubKey(), scopeNp2wkh + scopeP2wkh, nil
}

func printScopeInfo(name string, w *wallet.Wallet,
	scopes []waddrmgr.KeyScope) (string, error) {

	scopeInfo := ""
	for _, scope := range scopes {
		props, err := w.AccountProperties(scope, defaultAccount)
		if err != nil {
			return "", fmt.Errorf("error fetching account "+
				"properties: %v", err)
		}
		scopeInfo += fmt.Sprintf(
			keyScopeformat, scope.Purpose, scope.Coin, name,
			props.InternalKeyCount, name, props.ExternalKeyCount,
		)
	}

	return scopeInfo, nil
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
			return fmt.Errorf("namespace '%s' does not exist",
				waddrmgrNamespaceKey)
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
