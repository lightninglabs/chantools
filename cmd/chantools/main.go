package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"syscall"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btclog"
	"github.com/guggero/chantools/dataformat"
	"github.com/guggero/chantools/lnd"
	"github.com/jessevdk/go-flags"
	"github.com/lightningnetwork/lnd/build"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/channeldb"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	defaultAPIURL = "https://blockstream.info/api"
	version       = "0.7.1"
)

var (
	Commit = ""
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

	log.Infof("chantools version v%s commit %s", version, Commit)
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
		"sweeptimelockmanual", "Sweep the force-closed state of a "+
			"single channel manually if only a channel backup "+
			"file is available", "", &sweepTimeLockManualCommand{},
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
	_, _ = parser.AddCommand(
		"compactdb", "Open a source channel.db database file in safe/"+
			"read-only mode and copy it to a fresh database, "+
			"compacting it in the process.", "",
		&compactDBCommand{},
	)
	_, _ = parser.AddCommand(
		"vanitygen", "Generate a seed with a custom lnd node identity "+
			"public key that starts with the given prefix.", "",
		&vanityGenCommand{},
	)
	_, _ = parser.AddCommand(
		"rescuefunding", "Rescue funds locked in a funding multisig "+
			"output that never resulted in a proper channel. This "+
			"is the command the initiator of the channel needs to "+
			"run.", "",
		&rescueFundingCommand{},
	)
	_, _ = parser.AddCommand(
		"signrescuefunding", "Rescue funds locked in a funding "+
			"multisig output that never resulted in a proper "+
			"channel. This is the command the remote node (the non"+
			"-initiator) of the channel needs to run.", "",
		&signRescueFundingCommand{},
	)
	_, _ = parser.AddCommand(
		"removechannel", "Remove a single channel from the given "+
			"channel DB.", "", &removeChannelCommand{},
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
		db, err := lnd.OpenDB(cfg.FromChannelDB, true)
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

func passwordFromConsole(userQuery string) ([]byte, error) {
	// Read from terminal (if there is one).
	if terminal.IsTerminal(int(syscall.Stdin)) { // nolint
		fmt.Print(userQuery)
		pw, err := terminal.ReadPassword(int(syscall.Stdin)) // nolint
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
