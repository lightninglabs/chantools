package main

import (
	"fmt"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/lnd"
)

const (
	defaultRecoveryWindow = 2500
	defaultRescanFrom     = 500000
)

type genImportScriptCommand struct {
	RootKey        string `long:"rootkey" description:"BIP32 HD root key to use. Leave empty to prompt for lnd 24 word aezeed."`
	Format         string `long:"format" description:"The format of the generated import script. Currently supported are: bitcoin-cli, bitcoin-cli-watchonly, bitcoin-importwallet."`
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

	fmt.Printf("# Wallet dump created by chantools on %s\n",
		time.Now().UTC())

	// Determine the format.
	var printFn func(*hdkeychain.ExtendedKey, uint32, uint32) error
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

	// External branch first (m/84'/<coinType>'/0'/0/x).
	for i := uint32(0); i < c.RecoveryWindow; i++ {
		derivedKey, err := lnd.DeriveChildren(extendedKey, []uint32{
			lnd.HardenedKeyStart + uint32(84),
			lnd.HardenedKeyStart + chainParams.HDCoinType,
			lnd.HardenedKeyStart + uint32(0),
			0,
			i,
		})
		if err != nil {
			return err
		}
		err = printFn(derivedKey, 0, i)
		if err != nil {
			return err
		}
	}

	// Now the internal branch (m/84'/<coinType>'/0'/1/x).
	for i := uint32(0); i < c.RecoveryWindow; i++ {
		derivedKey, err := lnd.DeriveChildren(extendedKey, []uint32{
			lnd.HardenedKeyStart + uint32(84),
			lnd.HardenedKeyStart + chainParams.HDCoinType,
			lnd.HardenedKeyStart + uint32(0),
			1,
			i,
		})
		if err != nil {
			return err
		}
		err = printFn(derivedKey, 1, i)
		if err != nil {
			return err
		}
	}

	fmt.Printf("bitcoin-cli rescanblockchain %d\n", c.RescanFrom)
	return nil
}

func printBitcoinCli(hdKey *hdkeychain.ExtendedKey, branch,
	index uint32) error {

	privKey, err := hdKey.ECPrivKey()
	if err != nil {
		return fmt.Errorf("could not derive private key: %v",
			err)
	}
	wif, err := btcutil.NewWIF(privKey, chainParams, true)
	if err != nil {
		return fmt.Errorf("could not encode WIF: %v", err)
	}
	fmt.Printf("bitcoin-cli importprivkey %s \"m/84'/%d'/0'/%d/%d/"+
		"\" false\n", wif.String(), chainParams.HDCoinType, branch,
		index)
	return nil
}

func printBitcoinCliWatchOnly(hdKey *hdkeychain.ExtendedKey, branch,
	index uint32) error {

	pubKey, err := hdKey.ECPubKey()
	if err != nil {
		return fmt.Errorf("could not derive private key: %v",
			err)
	}
	fmt.Printf("bitcoin-cli importpubkey %x \"m/84'/%d'/0'/%d/%d/"+
		"\" false\n", pubKey.SerializeCompressed(),
		chainParams.HDCoinType, branch, index)
	return nil
}

func printBitcoinImportWallet(hdKey *hdkeychain.ExtendedKey, branch,
	index uint32) error {

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

	fmt.Printf("%s 1970-01-01T00:00:01Z label=m/84'/%d'/0'/%d/%d/ "+
		"# addr=%s", wif.String(), chainParams.HDCoinType, branch,
		index, addr.EncodeAddress(),
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
