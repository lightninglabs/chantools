package main

import (
	"fmt"
	"os"
	"time"

	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/btc"
	"github.com/guggero/chantools/lnd"
)

const (
	defaultRecoveryWindow = 2500
	defaultRescanFrom     = 500000
)

type genImportScriptCommand struct {
	RootKey        string `long:"rootkey" description:"BIP32 HD root key to use. Leave empty to prompt for lnd 24 word aezeed."`
	Format         string `long:"format" description:"The format of the generated import script. Currently supported are: bitcoin-cli, bitcoin-cli-watchonly, bitcoin-importwallet."`
	LndPaths       bool   `long:"lndpaths" description:"Use all derivation paths that lnd uses. Results in a large number of results. Cannot be used in conjunction with --derivationpath."`
	DerivationPath string `long:"derivationpath" description:"Use one specific derivation path. Specify the first levels of the derivation path before any internal/external branch. Cannot be used in conjunction with --lndpaths. (default m/84'/0'/0')"`
	RecoveryWindow uint32 `long:"recoverywindow" description:"The number of keys to scan per internal/external branch. The output will consist of double this amount of keys. (default 2500)"`
	RescanFrom     uint32 `long:"rescanfrom" description:"The block number to rescan from. Will be set automatically from the wallet birthday if the lnd 24 word aezeed is entered. (default 500000)"`
}

func (c *genImportScriptCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	var (
		extendedKey *hdkeychain.ExtendedKey
		err         error
		birthday    time.Time
		strPaths    []string
		paths       [][]uint32
	)

	// Check that root key is valid or fall back to console input.
	switch {
	case c.RootKey != "":
		extendedKey, err = hdkeychain.NewKeyFromString(c.RootKey)
		if err != nil {
			return fmt.Errorf("error reading root key: %v", err)
		}

	default:
		extendedKey, birthday, err = lnd.ReadAezeed(
			chainParams,
		)
		if err != nil {
			return fmt.Errorf("error reading root key: %v", err)
		}
		// The btcwallet gives the birthday a slack of 48 hours, let's
		// do the same.
		c.RescanFrom = btc.SeedBirthdayToBlock(
			chainParams, birthday.Add(-48*time.Hour),
		)
	}

	// Set default values.
	if c.RecoveryWindow == 0 {
		c.RecoveryWindow = defaultRecoveryWindow
	}
	if c.RescanFrom == 0 {
		c.RescanFrom = defaultRescanFrom
	}

	// Decide what derivation path(s) to use.
	switch {
	default:
		c.DerivationPath = lnd.WalletDefaultDerivationPath
		fallthrough

	case c.DerivationPath != "":
		derivationPath, err := lnd.ParsePath(c.DerivationPath)
		if err != nil {
			return fmt.Errorf("error parsing path: %v", err)
		}
		strPaths = []string{c.DerivationPath}
		paths = [][]uint32{derivationPath}

	case c.LndPaths && c.DerivationPath != "":
		return fmt.Errorf("cannot use --lndpaths and --derivationpath " +
			"at the same time")

	case c.LndPaths:
		strPaths, paths, err = lnd.AllDerivationPaths(chainParams)
		if err != nil {
			return fmt.Errorf("error getting lnd paths: %v", err)
		}
	}

	exporter := btc.ParseFormat(c.Format)
	err = btc.ExportKeys(
		extendedKey, strPaths, paths, chainParams, c.RecoveryWindow,
		c.RescanFrom, exporter, os.Stdout,
	)
	if err != nil {
		return fmt.Errorf("error exporting keys: %v", err)
	}

	return nil
}
