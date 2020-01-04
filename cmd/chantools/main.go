package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/guggero/chantools/dataformat"
	"github.com/jessevdk/go-flags"
	"github.com/lightningnetwork/lnd/build"
	"github.com/lightningnetwork/lnd/channeldb"
)

const (
	defaultApiUrl = "https://blockstream.info/api"
)

type config struct {
	Testnet         bool   `long:"testnet" description:"Set to true if testnet parameters should be used."`
	ApiUrl          string `long:"apiurl" description:"API URL to use (must be esplora compatible)."`
	ListChannels    string `long:"listchannels" description:"The channel input is in the format of lncli's listchannels format. Specify '-' to read from stdin."`
	PendingChannels string `long:"pendingchannels" description:"The channel input is in the format of lncli's pendingchannels format. Specify '-' to read from stdin."`
	FromSummary     string `long:"fromsummary" description:"The channel input is in the format of this tool's channel summary. Specify '-' to read from stdin."`
	FromChannelDB   string `long:"fromchanneldb" description:"The channel input is in the format of an lnd channel.db file."`
}

var (
	logWriter = build.NewRotatingLogWriter()
	log       = build.NewSubLogger("CHAN", logWriter.GenSubLogger)
	cfg       = &config{
		ApiUrl: defaultApiUrl,
	}
	chainParams = &chaincfg.MainNetParams
)

func main() {
	err := Main()
	if err == nil {
		return
	}

	_, ok := err.(*flags.Error)
	if !ok {
		fmt.Printf("Error running chantools: %v\n", err)
	}
	os.Exit(0)
}

func Main() error {
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

	_, err := parser.Parse()
	return err
}

func parseInput(cfg *config) ([]*dataformat.SummaryEntry, error) {
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
		db, err := channeldb.Open(cfg.FromChannelDB)
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

func setupChainParams(cfg *config) {
	if cfg.Testnet {
		chainParams = &chaincfg.TestNet3Params
	}
}

func setupLogging() {
	logWriter.RegisterSubLogger("CHAN", log)
	err := logWriter.InitLogRotator("./results/chantools.log", 10, 3)
	if err != nil {
		panic(err)
	}
	err = build.ParseAndSetDebugLevels("trace", logWriter)
	if err != nil {
		panic(err)
	}
}
