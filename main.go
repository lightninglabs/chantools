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
)

func Main() error {
	setupLogging()

	// Parse command line.
	parser := flags.NewParser(cfg, flags.Default)
	_, _ = parser.AddCommand(
		"summary", "Compile a summary about the current state of "+
			"channels", "", &summaryCommand{},
	)
	_, _ = parser.AddCommand(
		"rescueclosed", "Try finding the private keys for funds that "+
			"are in outputs of remotely force-closed channels", "",
		&rescueClosedCommand{},
	)
	_, _ = parser.AddCommand(
		"forceclose", "Force-close the last state that is in the "+
			"channel.db provided", "",
		&forceCloseCommand{},
	)
	_, _ = parser.AddCommand(
		"sweeptimelock", "Sweep the force-closed state after the time "+
			"lock has expired", "",
		&sweepTimeLockCommand{},
	)
	_, _ = parser.AddCommand(
		"dumpchannels", "Dump all channel information from lnd's "+
			"channel database", "",
		&dumpChannelsCommand{},
	)

	_, err := parser.Parse()
	return err
}

type summaryCommand struct{}

func (c *summaryCommand) Execute(_ []string) error {
	// Parse channel entries from any of the possible input files.
	entries, err := ParseInput(cfg)
	if err != nil {
		return err
	}
	return summarizeChannels(cfg.ApiUrl, entries)
}

type rescueClosedCommand struct {
	RootKey   string `long:"rootkey" description:"BIP32 HD root key to use."`
	ChannelDB string `long:"channeldb" description:"The lnd channel.db file to use for rescuing force-closed channels."`
}

func (c *rescueClosedCommand) Execute(_ []string) error {
	// Check that root key is valid.
	if c.RootKey == "" {
		return fmt.Errorf("root key is required")
	}
	extendedKey, err := hdkeychain.NewKeyFromString(c.RootKey)
	if err != nil {
		return fmt.Errorf("error parsing root key: %v", err)
	}

	// Check that we have a channel DB.
	if c.ChannelDB == "" {
		return fmt.Errorf("rescue DB is required")
	}
	db, err := channeldb.Open(path.Dir(c.ChannelDB))
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %v", err)
	}

	// Parse channel entries from any of the possible input files.
	entries, err := ParseInput(cfg)
	if err != nil {
		return err
	}
	return rescueClosedChannels(extendedKey, entries, db)
}

type forceCloseCommand struct {
	RootKey   string `long:"rootkey" description:"BIP32 HD root key to use."`
	ChannelDB string `long:"channeldb" description:"The lnd channel.db file to use for force-closing channels."`
	Publish   bool   `long:"publish" description:"Should the force-closing TX be published to the chain API?"`
}

func (c *forceCloseCommand) Execute(_ []string) error {
	// Check that root key is valid.
	if c.RootKey == "" {
		return fmt.Errorf("root key is required")
	}
	extendedKey, err := hdkeychain.NewKeyFromString(c.RootKey)
	if err != nil {
		return fmt.Errorf("error parsing root key: %v", err)
	}
	// Check that we have a channel DB.
	if c.ChannelDB == "" {
		return fmt.Errorf("rescue DB is required")
	}
	db, err := channeldb.Open(path.Dir(c.ChannelDB))
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %v", err)
	}

	// Parse channel entries from any of the possible input files.
	entries, err := ParseInput(cfg)
	if err != nil {
		return err
	}
	return forceCloseChannels(extendedKey, entries, db, c.Publish)
}

type sweepTimeLockCommand struct {
	RootKey     string `long:"rootkey" description:"BIP32 HD root key to use."`
	Publish     bool   `long:"publish" description:"Should the sweep TX be published to the chain API?"`
	SweepAddr   string `long:"sweepaddr" description:"The address the funds should be sweeped to"`
	MaxCsvLimit int    `long:"maxcsvlimit" description:"Maximum CSV limit to use. (default 2000)"`
}

func (c *sweepTimeLockCommand) Execute(_ []string) error {
	// Check that root key is valid.
	if c.RootKey == "" {
		return fmt.Errorf("root key is required")
	}
	extendedKey, err := hdkeychain.NewKeyFromString(c.RootKey)
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

	// Set default value
	if c.MaxCsvLimit == 0 {
		c.MaxCsvLimit = 2000
	}
	return sweepTimeLock(
		extendedKey, cfg.ApiUrl, entries, c.SweepAddr, c.MaxCsvLimit,
		c.Publish,
	)
}

type dumpChannelsCommand struct {
	ChannelDB string `long:"channeldb" description:"The lnd channel.db file to dump the channels from."`
}

func (c *dumpChannelsCommand) Execute(_ []string) error {
	// Check that we have a channel DB.
	if c.ChannelDB == "" {
		return fmt.Errorf("rescue DB is required")
	}
	db, err := channeldb.Open(path.Dir(c.ChannelDB))
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %v", err)
	}
	return dumpChannelInfo(db)
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
