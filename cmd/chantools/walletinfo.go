package main

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcwallet/waddrmgr"
	"github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/walletdb"
	_ "github.com/btcsuite/btcwallet/walletdb/bdb"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/spf13/cobra"
)

const (
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
	defaultAccount = uint32(waddrmgr.DefaultAccountNum)
)

type walletInfoCommand struct {
	WalletDB    string
	WithRootKey bool
	DumpAddrs   bool

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
in the wallet.db.
In case lnd was started with "--noseedbackup=true" your wallet has the default
password. To unlock the wallet set the environment variable WALLET_PASSWORD="-"
or simply press <enter> without entering a password when being prompted.`,
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
	cc.cmd.Flags().BoolVar(
		&cc.DumpAddrs, "dumpaddrs", false, "print all addresses, "+
			"including private keys",
	)

	return cc.cmd
}

func (c *walletInfoCommand) Execute(_ *cobra.Command, _ []string) error {
	// Check that we have a wallet DB.
	if c.WalletDB == "" {
		return fmt.Errorf("wallet DB is required")
	}

	w, privateWalletPw, cleanup, err := lnd.OpenWallet(
		c.WalletDB, chainParams,
	)
	if err != nil {
		return fmt.Errorf("error opening wallet file '%s': %w",
			c.WalletDB, err)
	}

	defer func() {
		if err := cleanup(); err != nil {
			log.Errorf("error closing wallet: %v", err)
		}
	}()

	// Print the wallet info and if requested the root key.
	identityKey, scopeInfo, err := walletInfo(w, c.DumpAddrs)
	if err != nil {
		return err
	}
	rootKey := na
	if c.WithRootKey {
		masterHDPrivKey, err := lnd.DecryptWalletRootKey(
			w.Database(), privateWalletPw,
		)
		if err != nil {
			return err
		}
		rootKey = string(masterHDPrivKey)
	}

	result := fmt.Sprintf(
		walletInfoFormat, identityKey.SerializeCompressed(), rootKey,
		scopeInfo,
	)

	fmt.Println(result)

	// For the tests, also log as trace level which is disabled by default.
	log.Tracef(result)

	return nil
}

func walletInfo(w *wallet.Wallet, dumpAddrs bool) (*btcec.PublicKey, string,
	error) {

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

	scopeAddrs := "\n"
	if dumpAddrs {
		printAddr := func(a waddrmgr.ManagedAddress) error {
			pka, ok := a.(waddrmgr.ManagedPubKeyAddress)
			if !ok {
				return fmt.Errorf("key is not a managed pubkey")
			}

			privKey, err := pka.PrivKey()
			if err != nil {
				return fmt.Errorf("error deriving priv key: %w",
					err)
			}

			scope, path, _ := pka.DerivationInfo()
			scopeAddrs += fmt.Sprintf(
				"path=m/%d'/%d'/%d'/%d/%d, pubkey=%x, "+
					"addr=%s, hash160=%x, priv=%x\n",
				scope.Purpose, scope.Coin, path.InternalAccount,
				path.Branch, path.Index,
				pka.PubKey().SerializeCompressed(),
				pka.Address().String(), a.AddrHash(),
				privKey.Serialize(),
			)
			return nil
		}
		for _, mgr := range w.Manager.ActiveScopedKeyManagers() {
			var addrs []waddrmgr.ManagedAddress
			err = walletdb.View(
				w.Database(), func(tx walletdb.ReadTx) error {
					waddrmgrNs := tx.ReadBucket(
						lnd.WaddrmgrNamespaceKey,
					)

					return mgr.ForEachAccountAddress(
						waddrmgrNs, 0,
						func(a waddrmgr.ManagedAddress) error {
							addrs = append(addrs, a)
							return nil
						},
					)
				},
			)
			if err != nil {
				return nil, "", err
			}

			for _, addr := range addrs {
				if err := printAddr(addr); err != nil {
					return nil, "", err
				}
			}
		}
	}

	return idPrivKey.PubKey(), scopeNp2wkh + scopeP2wkh + scopeAddrs, nil
}

func printScopeInfo(name string, w *wallet.Wallet,
	scopes []waddrmgr.KeyScope) (string, error) {

	scopeInfo := ""
	for _, scope := range scopes {
		props, err := w.AccountProperties(scope, defaultAccount)
		if err != nil {
			return "", fmt.Errorf("error fetching account "+
				"properties: %w", err)
		}
		scopeInfo += fmt.Sprintf(
			keyScopeformat, scope.Purpose, scope.Coin, name,
			props.InternalKeyCount, name, props.ExternalKeyCount,
		)
	}

	return scopeInfo, nil
}
