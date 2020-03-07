package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/lightningnetwork/lnd/chanbackup"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btclog"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/dataformat"
	"github.com/jessevdk/go-flags"
	"github.com/lightningnetwork/lnd/aezeed"
	"github.com/lightningnetwork/lnd/build"
	"github.com/lightningnetwork/lnd/channeldb"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	defaultAPIURL = "https://blockstream.info/api"
)

type config struct {
	Testnet         bool   `long:"testnet" description:"Set to true if testnet parameters should be used."`
	Regtest         bool   `long:"regtest" description:"Set to true if regtest parameters should be used."`
	APIURL          string `long:"apiurl" description:"API URL to use (must be esplora compatible)."`
	ListChannels    string `long:"listchannels" description:"The channel input is in the format of lncli's listchannels format. Specify '-' to read from stdin."`
	PendingChannels string `long:"pendingchannels" description:"The channel input is in the format of lncli's pendingchannels format. Specify '-' to read from stdin."`
	FromSummary     string `long:"fromsummary" description:"The channel input is in the format of this tool's channel summary. Specify '-' to read from stdin."`
	FromChannelDB   string `long:"fromchanneldb" description:"The channel input is in the format of an lnd channel.db file."`
}

var (
	logWriter = build.NewRotatingLogWriter()
	log       = build.NewSubLogger("CHAN", logWriter.GenSubLogger)
	cfg       = &config{
		APIURL: defaultAPIURL,
	}
	chainParams = &chaincfg.MainNetParams
)

func main() {
	err := runCommandParser()
	if err == nil {
		return
	}

	_, ok := err.(*flags.Error)
	if !ok {
		fmt.Printf("Error running chantools: %v\n", err)
	}
	os.Exit(0)
}

func runCommandParser() error {
	setupLogging()

	// Parse command line.
	parser := flags.NewParser(cfg, flags.Default)
	_, _ = parser.AddCommand(
		"summary", "Compile a summary about the current state of "+
			"channels.", "", &summaryCommand{},
	)
	_, _ = parser.AddCommand(
		"rescueclosed", "Try finding the private keys for funds that "+
			"are in outputs of remotely force-closed channels.", "",
		&rescueClosedCommand{},
	)
	_, _ = parser.AddCommand(
		"forceclose", "Force-close the last state that is in the "+
			"channel.db provided.", "", &forceCloseCommand{},
	)
	_, _ = parser.AddCommand(
		"sweeptimelock", "Sweep the force-closed state after the time "+
			"lock has expired.", "", &sweepTimeLockCommand{},
	)
	_, _ = parser.AddCommand(
		"dumpchannels", "Dump all channel information from lnd's "+
			"channel database.", "", &dumpChannelsCommand{},
	)
	_, _ = parser.AddCommand(
		"showrootkey", "Extract and show the BIP32 HD root key from "+
			"the 24 word lnd aezeed.", "", &showRootKeyCommand{},
	)
	_, _ = parser.AddCommand(
		"dumpbackup", "Dump the content of a channel.backup file.", "",
		&dumpBackupCommand{},
	)
	_, _ = parser.AddCommand(
		"derivekey", "Derive a key with a specific derivation path "+
			"from the BIP32 HD root key.", "", &deriveKeyCommand{},
	)
	_, _ = parser.AddCommand(
		"filterbackup", "Filter an lnd channel.backup file and "+
			"remove certain channels.", "", &filterBackupCommand{},
	)
	_, _ = parser.AddCommand(
		"fixoldbackup", "Fixes an old channel.backup file that is "+
			"affected by the lnd issue #3881 (unable to derive "+
			"shachain root key).", "", &fixOldBackupCommand{},
	)
	_, _ = parser.AddCommand(
		"genimportscript", "Generate a script containing the on-chain "+
			"keys of an lnd wallet that can be imported into "+
			"other software like bitcoind.", "",
		&genImportScriptCommand{},
	)
	_, _ = parser.AddCommand(
		"walletinfo", "Shows relevant information about an lnd "+
			"wallet.db file and optionally extracts the BIP32 HD "+
			"root key.", "", &walletInfoCommand{},
	)
	_, _ = parser.AddCommand(
		"chanbackup", "Create a channel.backup file from a channel "+
			"database.", "", &chanBackupCommand{},
	)

	_, err := parser.Parse()
	return err
}

func parseInputType(cfg *config) ([]*dataformat.SummaryEntry, error) {
	var (
		content []byte
		err     error
		target  dataformat.InputFile
	)

	switch {
	case cfg.ListChannels != "":
		content, err = readInput(cfg.ListChannels)
		target = &dataformat.ListChannelsFile{}

	case cfg.PendingChannels != "":
		content, err = readInput(cfg.PendingChannels)
		target = &dataformat.PendingChannelsFile{}

	case cfg.FromSummary != "":
		content, err = readInput(cfg.FromSummary)
		target = &dataformat.SummaryEntryFile{}

	case cfg.FromChannelDB != "":
		db, err := channeldb.Open(
			cfg.FromChannelDB,
			channeldb.OptionSetSyncFreelist(true),
			channeldb.OptionReadOnly(true),
		)
		if err != nil {
			return nil, fmt.Errorf("error opening channel DB: %v",
				err)
		}
		target = &dataformat.ChannelDBFile{DB: db}
		return target.AsSummaryEntries()

	default:
		return nil, fmt.Errorf("an input file must be specified")
	}

	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	err = decoder.Decode(&target)
	if err != nil {
		return nil, err
	}
	return target.AsSummaryEntries()
}

func readInput(input string) ([]byte, error) {
	if strings.TrimSpace(input) == "-" {
		return ioutil.ReadAll(os.Stdin)
	}
	return ioutil.ReadFile(input)
}

func rootKeyFromConsole() (*hdkeychain.ExtendedKey, time.Time, error) {
	// We'll now prompt the user to enter in their 24-word mnemonic.
	fmt.Printf("Input your 24-word mnemonic separated by spaces: ")
	reader := bufio.NewReader(os.Stdin)
	mnemonicStr, err := reader.ReadString('\n')
	if err != nil {
		return nil, time.Unix(0, 0), err
	}

	// We'll trim off extra spaces, and ensure the mnemonic is all
	// lower case, then populate our request.
	mnemonicStr = strings.TrimSpace(mnemonicStr)
	mnemonicStr = strings.ToLower(mnemonicStr)

	cipherSeedMnemonic := strings.Split(mnemonicStr, " ")

	fmt.Println()

	if len(cipherSeedMnemonic) != 24 {
		return nil, time.Unix(0, 0), fmt.Errorf("wrong cipher seed "+
			"mnemonic length: got %v words, expecting %v words",
			len(cipherSeedMnemonic), 24)
	}

	// Additionally, the user may have a passphrase, that will also
	// need to be provided so the daemon can properly decipher the
	// cipher seed.
	fmt.Printf("Input your cipher seed passphrase (press enter if " +
		"your seed doesn't have a passphrase): ")
	passphrase, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		return nil, time.Unix(0, 0), err
	}

	var mnemonic aezeed.Mnemonic
	copy(mnemonic[:], cipherSeedMnemonic)

	// If we're unable to map it back into the ciphertext, then either the
	// mnemonic is wrong, or the passphrase is wrong.
	cipherSeed, err := mnemonic.ToCipherSeed(passphrase)
	if err != nil {
		return nil, time.Unix(0, 0), fmt.Errorf("failed to decrypt "+
			"seed with passphrase: %v", err)
	}
	rootKey, err := hdkeychain.NewMaster(cipherSeed.Entropy[:], chainParams)
	if err != nil {
		return nil, time.Unix(0, 0), fmt.Errorf("failed to derive " +
			"master extended key")
	}
	return rootKey, cipherSeed.BirthdayTime(), nil
}

func passwordFromConsole(userQuery string) ([]byte, error) {
	// Read from terminal (if there is one).
	if terminal.IsTerminal(syscall.Stdin) {
		fmt.Print(userQuery)
		pw, err := terminal.ReadPassword(syscall.Stdin)
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
	return pw, nil
}

func setupChainParams(cfg *config) {
	switch {
	case cfg.Testnet:
		chainParams = &chaincfg.TestNet3Params

	case cfg.Regtest:
		chainParams = &chaincfg.RegressionNetParams

	default:
		chainParams = &chaincfg.MainNetParams
	}
}

func setupLogging() {
	setSubLogger("CHAN", log)
	addSubLogger("CHDB", channeldb.UseLogger)
	addSubLogger("BCKP", chanbackup.UseLogger)
	err := logWriter.InitLogRotator("./results/chantools.log", 10, 3)
	if err != nil {
		panic(err)
	}
	err = build.ParseAndSetDebugLevels("trace", logWriter)
	if err != nil {
		panic(err)
	}
}

// addSubLogger is a helper method to conveniently create and register the
// logger of one or more sub systems.
func addSubLogger(subsystem string, useLoggers ...func(btclog.Logger)) {
	// Create and register just a single logger to prevent them from
	// overwriting each other internally.
	logger := build.NewSubLogger(subsystem, logWriter.GenSubLogger)
	setSubLogger(subsystem, logger, useLoggers...)
}

// setSubLogger is a helper method to conveniently register the logger of a sub
// system.
func setSubLogger(subsystem string, logger btclog.Logger,
	useLoggers ...func(btclog.Logger)) {

	logWriter.RegisterSubLogger(subsystem, logger)
	for _, useLogger := range useLoggers {
		useLogger(logger)
	}
}

func noConsole() ([]byte, error) {
	return nil, fmt.Errorf("wallet db requires console access")
}
