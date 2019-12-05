package chantools

import (
	"fmt"
	"path"

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
	FromChannelDB   string `long:"fromchanneldb" description:"The channel input is in the format of an lnd channel.db file."`
	ChannelDB       string `long:"channeldb" description:"The lnd channel.db file to use for rescuing or force-closing channels."`
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
	_, _ = parser.AddCommand(
		"forceclose", "Force-close the last state that is in the " +
			"channel.db provided", "",
		&forceCloseCommand{},
	)
	_, _ = parser.AddCommand(
		"sweeptimelock", "Sweep the force-closed state after the time " +
			"lock has expired", "",
		&sweepTimeLockCommand{},
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

	// Check that we have a channel DB.
	if cfg.ChannelDB == "" {
		return fmt.Errorf("rescue DB is required")
	}
	db, err := channeldb.Open(path.Dir(cfg.ChannelDB))
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

type forceCloseCommand struct {
	Publish bool `long:"publish" description:"Should the force-closing TX be published to the chain API?"`
}

func (c *forceCloseCommand) Execute(args []string) error {
	// Check that we have a channel DB.
	if cfg.ChannelDB == "" {
		return fmt.Errorf("rescue DB is required")
	}
	db, err := channeldb.Open(path.Dir(cfg.ChannelDB))
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %v", err)
	}

	// Parse channel entries from any of the possible input files.
	entries, err := ParseInput(cfg)
	if err != nil {
		return err
	}
	return forceCloseChannels(cfg, entries, db, c.Publish)
}

type sweepTimeLockCommand struct {
	Publish bool `long:"publish" description:"Should the sweep TX be published to the chain API?"`
	SweepAddr string `long:"sweepaddr" description:"The address the funds should be sweeped to"`
}

func (c *sweepTimeLockCommand) Execute(args []string) error {
	// Check that root key is valid.
	if cfg.RootKey == "" {
		return fmt.Errorf("root key is required")
	}
	_, err := hdkeychain.NewKeyFromString(cfg.RootKey)
	if err != nil {
		return fmt.Errorf("error parsing root key: %v", err)
	}
	
	// Make sure sweep addr is set.
	if c.SweepAddr == "" {
		return fmt.Errorf("sweep addr is required")
	}

	// Parse channel entries from any of the possible input files.
	entries, err := ParseInput(cfg)
	if err != nil {
		return err
	}
	return sweepTimeLock(cfg, entries, c.SweepAddr, c.Publish)
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
