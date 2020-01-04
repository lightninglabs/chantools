package chantools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"syscall"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/btc"
	"github.com/guggero/chantools/dataformat"
	"github.com/jessevdk/go-flags"
	"github.com/lightningnetwork/lnd/aezeed"
	"github.com/lightningnetwork/lnd/build"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/channeldb"
	"golang.org/x/crypto/ssh/terminal"
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
		return fmt.Errorf("channel DB is required")
	}
	db, err := channeldb.Open(path.Dir(c.ChannelDB))
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %v", err)
	}
	return dumpChannelInfo(db)
}

type showRootKeyCommand struct{}

func (c *showRootKeyCommand) Execute(_ []string) error {
	// We'll now prompt the user to enter in their 24-word mnemonic.
	fmt.Printf("Input your 24-word mnemonic separated by spaces: ")
	reader := bufio.NewReader(os.Stdin)
	mnemonicStr, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	// We'll trim off extra spaces, and ensure the mnemonic is all
	// lower case, then populate our request.
	mnemonicStr = strings.TrimSpace(mnemonicStr)
	mnemonicStr = strings.ToLower(mnemonicStr)

	cipherSeedMnemonic := strings.Split(mnemonicStr, " ")

	fmt.Println()

	if len(cipherSeedMnemonic) != 24 {
		return fmt.Errorf("wrong cipher seed mnemonic "+
			"length: got %v words, expecting %v words",
			len(cipherSeedMnemonic), 24)
	}

	// Additionally, the user may have a passphrase, that will also
	// need to be provided so the daemon can properly decipher the
	// cipher seed.
	fmt.Printf("Input your cipher seed passphrase (press enter if " +
		"your seed doesn't have a passphrase): ")
	passphrase, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		return err
	}

	var mnemonic aezeed.Mnemonic
	copy(mnemonic[:], cipherSeedMnemonic[:])

	// If we're unable to map it back into the ciphertext, then either the
	// mnemonic is wrong, or the passphrase is wrong.
	cipherSeed, err := mnemonic.ToCipherSeed(passphrase)
	if err != nil {
		return err
	}
	rootKey, err := hdkeychain.NewMaster(cipherSeed.Entropy[:], chainParams)
	if err != nil {
		return fmt.Errorf("failed to derive master extended key")
	}
	fmt.Printf("\nYour BIP32 HD root key is: %s\n", rootKey.String())
	return nil
}

type dumpBackupCommand struct {
	RootKey   string `long:"rootkey" description:"BIP32 HD root key of the wallet that was used to create the backup."`
	MultiFile string `long:"multi_file" description:"The lnd channel.backup file to dump."`
}

func (c *dumpBackupCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	// Check that root key is valid.
	if c.RootKey == "" {
		return fmt.Errorf("root key is required")
	}
	extendedKey, err := hdkeychain.NewKeyFromString(c.RootKey)
	if err != nil {
		return fmt.Errorf("error parsing root key: %v", err)
	}

	// Check that we have a backup file.
	if c.MultiFile == "" {
		return fmt.Errorf("backup file is required")
	}
	multiFile := chanbackup.NewMultiFile(c.MultiFile)
	multi, err := multiFile.ExtractMulti(&btc.ChannelBackupEncryptionRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	})
	if err != nil {
		return fmt.Errorf("could not extract multi file: %v", err)
	}
	return dumpChannelBackup(multi)
}

type deriveKeyCommand struct {
	RootKey string `long:"rootkey" description:"BIP32 HD root key to derive the key from."`
	Path    string `long:"path" description:"The BIP32 derivation path to derive. Must start with \"m/\"."`
	Neuter  bool   `long:"neuter" description:"Do not output the private key, just the public key."`
}

func (c *deriveKeyCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	// Check that root key is valid.
	if c.RootKey == "" {
		return fmt.Errorf("root key is required")
	}
	extendedKey, err := hdkeychain.NewKeyFromString(c.RootKey)
	if err != nil {
		return fmt.Errorf("error parsing root key: %v", err)
	}

	return deriveKey(extendedKey, c.Path, c.Neuter)
}

func ParseInput(cfg *config) ([]*dataformat.SummaryEntry, error) {
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
