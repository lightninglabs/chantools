package chantools

import (
	"fmt"
	
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/jessevdk/go-flags"
	"github.com/lightningnetwork/lnd/build"
	"github.com/lightningnetwork/lnd/channeldb"
)

const (
	defaultApiUrl = "https://blockstream.info/api"
)

type config struct {
	ApiUrl          string `long:"apiurl" description:"API URL to use (must be esplora compatible)."`
	RootKey         string `long:"rootkey" description:"BIP32 HD root key to use."`
	ListChannels    string `long:"listchannels" description:"The channel input is in the format of lncli's listchannels format. Specify '-' to read from stdin."`
	PendingChannels string `long:"pendingchannels" description:"The channel input is in the format of lncli's pendingchannels format. Specify '-' to read from stdin."`
	FromSummary     string `long:"fromsummary" description:"The channel input is in the format of this tool's channel summary. Specify '-' to read from stdin."`
	FromChannelDB   string `long:"fromchanneldb" description:"The channel input is in the format of an lnd channel.db file. Specify '-' to read from stdin."`
	RescueDB        string `long:"rescuedb" description:"The lnd channel.db file to use for rescuing remote force-closed channels."`
}

var (
	logWriter = build.NewRotatingLogWriter()
	log       = build.NewSubLogger("CHAN", logWriter.GenSubLogger)
	cfg       = &config{
		ApiUrl: defaultApiUrl,
	}
)

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
			"are in outputs of remotely force-closed channels", "",
		&rescueClosedCommand{},
	)

	_, err := parser.Parse()
	return err
}

type summaryCommand struct{}

func (c *summaryCommand) Execute(args []string) error {
	// Parse channel entries from any of the possible input files.
	entries, err := ParseInput(cfg)
	if err != nil {
		return err
	}
	return collectChanSummary(cfg, entries)
}

type rescueClosedCommand struct{}

func (c *rescueClosedCommand) Execute(args []string) error {
	// Check that root key is valid.
	if cfg.RootKey == "" {
		return fmt.Errorf("root key is required")
	}
	_, err := hdkeychain.NewKeyFromString(cfg.RootKey)
	if err != nil {
		return fmt.Errorf("error parsing root key: %v", err)
	}

	// Check that we have a rescue DB.
	if cfg.RescueDB == "" {
		return fmt.Errorf("rescue DB is required")
	}
	db, err := channeldb.Open(cfg.RescueDB)
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %v", err)
	}

	// Parse channel entries from any of the possible input files.
	entries, err := ParseInput(cfg)
	if err != nil {
		return err
	}
	return bruteForceChannels(cfg, entries, db)
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
