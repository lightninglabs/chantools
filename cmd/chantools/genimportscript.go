package main

import (
	"fmt"
	"os"
	"time"

	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/lnd"
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
	Stdout         bool

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
  in those private keys. NOTE: This will only work for legacy wallets and only
  for legacy, p2sh-segwit and bech32 (p2pkh, np2wkh and p2wkh) addresses. Use
  bitcoin-descriptors and a descriptor wallet for bech32m (p2tr).
* bitcoin-cli-watchonly: Does the same as bitcoin-cli but with the
  bitcoin-cli importpubkey command. That means, only the public keys are 
  imported into bitcoind to watch the UTXOs of those keys. The funds cannot be
  spent that way as they are watch-only.
* bitcoin-importwallet: Creates a text output that is compatible with
  bitcoind's importwallet command.
* electrum: Creates a text output that contains one private key per line with
  the address type as the prefix, the way Electrum expects them.
* bitcoin-descriptors: Create a list of bitcoin-cli importdescriptors commands
  that can be used in combination with a bitcoind full node that has a
  descriptor wallet to recover the funds locked in those private keys.
  NOTE: This will only work for descriptor wallets and only for
  p2sh-segwit, bech32 and bech32m (np2wkh, p2wkh and p2tr) addresses.`,
		Example: `chantools genimportscript --format bitcoin-cli \
	--recoverywindow 5000`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.Format, "format", "bitcoin-importwallet", "format of the "+
			"generated import script; currently supported are: "+
			"bitcoin-importwallet, bitcoin-cli, "+
			"bitcoin-cli-watchonly, bitcoin-descriptors and "+
			"electrum",
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
	cc.cmd.Flags().BoolVar(
		&cc.Stdout, "stdout", false, "write generated import script "+
			"to standard out instead of writing it to a file")

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
		return fmt.Errorf("error reading root key: %w", err)
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

	case c.DerivationPath == "-":
		strPaths = []string{""}
		paths = [][]uint32{{}}

	case c.DerivationPath != "":
		derivationPath, err := lnd.ParsePath(c.DerivationPath)
		if err != nil {
			return fmt.Errorf("error parsing path: %w", err)
		}
		strPaths = []string{c.DerivationPath}
		paths = [][]uint32{derivationPath}

	case c.LndPaths && c.DerivationPath != "":
		return fmt.Errorf("cannot use --lndpaths and --derivationpath " +
			"at the same time")

	case c.LndPaths:
		strPaths, paths, err = lnd.AllDerivationPaths(chainParams)
		if err != nil {
			return fmt.Errorf("error getting lnd paths: %w", err)
		}
	}

	writer := os.Stdout
	if !c.Stdout {
		fileName := fmt.Sprintf("results/genimportscript-%s.txt",
			time.Now().Format("2006-01-02-15-04-05"))
		log.Infof("Writing import script with format '%s' to %s",
			c.Format, fileName)

		var err error
		writer, err = os.Create(fileName)
		if err != nil {
			return fmt.Errorf("error creating result file %s: %w",
				fileName, err)
		}
	}

	exporter, err := btc.ParseFormat(c.Format)
	if err != nil {
		return fmt.Errorf("error parsing format: %w", err)
	}

	err = btc.ExportKeys(
		extendedKey, strPaths, paths, chainParams, c.RecoveryWindow,
		c.RescanFrom, exporter, writer,
	)
	if err != nil {
		return fmt.Errorf("error exporting keys: %w", err)
	}

	return nil
}
