package main

import (
	"fmt"
	"os"
	"time"

	"github.com/guggero/chantools/btc"
	"github.com/guggero/chantools/lnd"
	"github.com/spf13/cobra"
)

const (
	defaultRecoveryWindow = 2500
	defaultRescanFrom     = 500000
)

type genImportScriptCommand struct {
	Format         string
	LndPaths       bool
	DerivationPath string
	RecoveryWindow uint32
	RescanFrom     uint32

	rootKey *rootKey
	cmd     *cobra.Command
}

func newGenImportScriptCommand() *cobra.Command {
	cc := &genImportScriptCommand{}
	cc.cmd = &cobra.Command{
		Use: "genimportscript",
		Short: "Generate a script containing the on-chain " +
			"keys of an lnd wallet that can be imported into " +
			"other software like bitcoind",
		Long: `Generates a script that contains all on-chain private (or
public) keys derived from an lnd 24 word aezeed wallet. That script can then be
imported into other software like bitcoind.

The following script formats are currently supported:
* bitcoin-cli: Creates a list of bitcoin-cli importprivkey commands that can
  be used in combination with a bitcoind full node to recover the funds locked
  in those private keys.
* bitcoin-cli-watchonly: Does the same as bitcoin-cli but with the
  bitcoin-cli importpubkey command. That means, only the public keys are 
  imported into bitcoind to watch the UTXOs of those keys. The funds cannot be
  spent that way as they are watch-only.
* bitcoin-importwallet: Creates a text output that is compatible with
  bitcoind's importwallet command.`,
		Example: `chantools genimportscript --format bitcoin-cli \
	--recoverywindow 5000`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.Format, "format", "bitcoin-importwallet", "format of the "+
			"generated import script; currently supported are: "+
			"bitcoin-importwallet, bitcoin-cli and "+
			"bitcoin-cli-watchonly",
	)
	cc.cmd.Flags().BoolVar(
		&cc.LndPaths, "lndpaths", false, "use all derivation paths "+
			"that lnd used; results in a large number of results; "+
			"cannot be used in conjunction with --derivationpath",
	)
	cc.cmd.Flags().StringVar(
		&cc.DerivationPath, "derivationpath", "", "use one specific "+
			"derivation path; specify the first levels of the "+
			"derivation path before any internal/external branch; "+
			"Cannot be used in conjunction with --lndpaths",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.RecoveryWindow, "recoverywindow", defaultRecoveryWindow,
		"number of keys to scan per internal/external branch; output "+
			"will consist of double this amount of keys",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.RescanFrom, "rescanfrom", defaultRescanFrom, "block "+
			"number to rescan from; will be set automatically "+
			"from the wallet birthday if the lnd 24 word aezeed "+
			"is entered",
	)

	cc.rootKey = newRootKey(cc.cmd, "decrypting the backup")

	return cc.cmd
}

func (c *genImportScriptCommand) Execute(_ *cobra.Command, _ []string) error {
	var (
		strPaths []string
		paths    [][]uint32
	)

	extendedKey, birthday, err := c.rootKey.readWithBirthday()
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	// The btcwallet gives the birthday a slack of 48 hours, let's do the
	// same.
	if !birthday.IsZero() {
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
