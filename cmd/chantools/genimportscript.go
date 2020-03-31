package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/lnd"
)

const (
	defaultRecoveryWindow = 2500
	defaultRescanFrom     = 500000
	defaultDerivationPath = "m/84'/0'/0'"
)

type genImportScriptCommand struct {
	RootKey        string `long:"rootkey" description:"BIP32 HD root key to use. Leave empty to prompt for lnd 24 word aezeed."`
	Format         string `long:"format" description:"The format of the generated import script. Currently supported are: bitcoin-cli, bitcoin-cli-watchonly, bitcoin-importwallet."`
	DerivationPath string `long:"derivationpath" description:"The first levels of the derivation path before any internal/external branch. (default m/84'/0'/0')"`
	RecoveryWindow uint32 `long:"recoverywindow" description:"The number of keys to scan per internal/external branch. The output will consist of double this amount of keys. (default 2500)"`
	RescanFrom     uint32 `long:"rescanfrom" description:"The block number to rescan from. Will be set automatically from the wallet birthday if the lnd 24 word aezeed is entered. (default 500000)"`
}

func (c *genImportScriptCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	var (
		extendedKey *hdkeychain.ExtendedKey
		err         error
		birthday    time.Time
	)

	// Check that root key is valid or fall back to console input.
	switch {
	case c.RootKey != "":
		extendedKey, err = hdkeychain.NewKeyFromString(c.RootKey)
		if err != nil {
			return fmt.Errorf("error reading root key: %v", err)
		}

	default:
		extendedKey, birthday, err = rootKeyFromConsole()
		if err != nil {
			return fmt.Errorf("error reading root key: %v", err)
		}
		// The btcwallet gives the birthday a slack of 48 hours, let's
		// do the same.
		c.RescanFrom = seedBirthdayToBlock(birthday.Add(-48 * time.Hour))
	}

	// Set default values.
	if c.RecoveryWindow == 0 {
		c.RecoveryWindow = defaultRecoveryWindow
	}
	if c.RescanFrom == 0 {
		c.RescanFrom = defaultRescanFrom
	}
	if c.DerivationPath == "" {
		c.DerivationPath = defaultDerivationPath
	}

	// Process derivation path
	levels := strings.Split(c.DerivationPath, "/")
	if len(levels) == 0 || levels[0] != "m" {
		return fmt.Errorf("error reading derivationpath: path \"%s\" not in "+
			"correct format, e.g. \"m/purpose'/coin_type'/account'\"", c.DerivationPath)
	}
	levels = levels[1:] // removes masterseed purposed "m"

	derivationPath := make([]uint32, len(levels))
	for i := range levels {
		unHardened := strings.TrimSuffix(levels[i], "'")
		d, err := strconv.Atoi(unHardened)
		if err != nil {
			return fmt.Errorf("error reading derivationpath: <%s> is not a valid "+
				"derivation", unHardened)
		}

		if levels[i] == unHardened {
			derivationPath[i] = uint32(d)
		} else {
			derivationPath[i] = lnd.HardenedKeyStart + uint32(d)
		}
	}

	fmt.Printf("# Wallet dump created by chantools on %s\n",
		time.Now().UTC())

	// Determine the format.
	var printFn func(*hdkeychain.ExtendedKey, string, uint32, uint32) error
	switch c.Format {
	default:
		fallthrough

	case "bitcoin-cli":
		printFn = printBitcoinCli
		fmt.Println("# Paste the following lines into a command line " +
			"window.")

	case "bitcoin-cli-watchonly":
		printFn = printBitcoinCliWatchOnly
		fmt.Println("# Paste the following lines into a command line " +
			"window.")

	case "bitcoin-importwallet":
		printFn = printBitcoinImportWallet
		fmt.Println("# Save this output to a file and use the " +
			"importwallet command of bitcoin core.")
	}

	// External branch first (<DerivationPath>/0/i).
	for i := uint32(0); i < c.RecoveryWindow; i++ {
		path := append(derivationPath, []uint32{0, i}...)
		derivedKey, err := lnd.DeriveChildren(extendedKey, path)
		if err != nil {
			return err
		}
		err = printFn(derivedKey, c.DerivationPath, 0, i)
		if err != nil {
			return err
		}
	}

	// Now the internal branch (<DerivationPath>/1/i).
	for i := uint32(0); i < c.RecoveryWindow; i++ {
		path := append(derivationPath, []uint32{1, i}...)
		derivedKey, err := lnd.DeriveChildren(extendedKey, path)
		if err != nil {
			return err
		}
		err = printFn(derivedKey, c.DerivationPath, 1, i)
		if err != nil {
			return err
		}
	}

	fmt.Printf("bitcoin-cli rescanblockchain %d\n", c.RescanFrom)
	return nil
}

func printBitcoinCli(hdKey *hdkeychain.ExtendedKey, path string,
	branch, index uint32) error {

	privKey, err := hdKey.ECPrivKey()
	if err != nil {
		return fmt.Errorf("could not derive private key: %v",
			err)
	}
	wif, err := btcutil.NewWIF(privKey, chainParams, true)
	if err != nil {
		return fmt.Errorf("could not encode WIF: %v", err)
	}
	fmt.Printf("bitcoin-cli importprivkey %s \"%s/%d/%d/"+
		"\" false\n", wif.String(), path, branch,
		index)
	return nil
}

func printBitcoinCliWatchOnly(hdKey *hdkeychain.ExtendedKey, path string,
	branch, index uint32) error {

	pubKey, err := hdKey.ECPubKey()
	if err != nil {
		return fmt.Errorf("could not derive private key: %v",
			err)
	}
	fmt.Printf("bitcoin-cli importpubkey %x \"%s/%d/%d/"+
		"\" false\n", pubKey.SerializeCompressed(),
		path, branch, index)
	return nil
}

func printBitcoinImportWallet(hdKey *hdkeychain.ExtendedKey, path string,
	branch, index uint32) error {

	privKey, err := hdKey.ECPrivKey()
	if err != nil {
		return fmt.Errorf("could not derive private key: %v",
			err)
	}
	wif, err := btcutil.NewWIF(privKey, chainParams, true)
	if err != nil {
		return fmt.Errorf("could not encode WIF: %v", err)
	}
	pubKey, err := hdKey.ECPubKey()
	if err != nil {
		return fmt.Errorf("could not derive private key: %v",
			err)
	}
	addrPubkey, err := btcutil.NewAddressPubKey(
		pubKey.SerializeCompressed(), chainParams,
	)
	if err != nil {
		return fmt.Errorf("could not create address: %v", err)
	}
	addr := addrPubkey.AddressPubKeyHash()

	fmt.Printf("%s 1970-01-01T00:00:01Z label=%s/%d/%d/ "+
		"# addr=%s\n", wif.String(), path, branch, index,
		addr.EncodeAddress(),
	)
	return nil
}

func seedBirthdayToBlock(birthdayTimestamp time.Time) uint32 {
	var genesisTimestamp time.Time
	switch chainParams.Name {
	case "mainnet":
		genesisTimestamp =
			chaincfg.MainNetParams.GenesisBlock.Header.Timestamp

	case "testnet":
		genesisTimestamp =
			chaincfg.TestNet3Params.GenesisBlock.Header.Timestamp

	default:
		panic(fmt.Errorf("unimplemented network %v", chainParams.Name))
	}

	// With the timestamps retrieved, we can estimate a block height by
	// taking the difference between them and dividing by the average block
	// time (10 minutes).
	return uint32(birthdayTimestamp.Sub(genesisTimestamp).Seconds() / 600)
}
