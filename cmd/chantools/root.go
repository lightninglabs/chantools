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
	"time"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btclog"
	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/dataformat"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/build"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/peer"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	defaultAPIURL = "https://blockstream.info/api"
	version       = "0.11.3"
	na            = "n/a"

	Commit = ""
)

var (
	Testnet bool
	Regtest bool

	logWriter   = build.NewRotatingLogWriter()
	log         = build.NewSubLogger("CHAN", genSubLogger(logWriter))
	chainParams = &chaincfg.MainNetParams
)

var rootCmd = &cobra.Command{
	Use:   "chantools",
	Short: "Chantools helps recover funds from lightning channels",
	Long: `This tool provides helper functions that can be used rescue
funds locked in lnd channels in case lnd itself cannot run properly anymore.
Complete documentation is available at https://github.com/lightninglabs/chantools/.`,
	Version: fmt.Sprintf("v%s, commit %s", version, Commit),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		switch {
		case Testnet:
			chainParams = &chaincfg.TestNet3Params

		case Regtest:
			chainParams = &chaincfg.RegressionNetParams

		default:
			chainParams = &chaincfg.MainNetParams
		}

		setupLogging()

		log.Infof("chantools version v%s commit %s", version,
			Commit)
	},
	DisableAutoGenTag: true,
}

func main() {
	rootCmd.PersistentFlags().BoolVarP(
		&Testnet, "testnet", "t", false, "Indicates if testnet "+
			"parameters should be used",
	)
	rootCmd.PersistentFlags().BoolVarP(
		&Regtest, "regtest", "r", false, "Indicates if regtest "+
			"parameters should be used",
	)

	rootCmd.AddCommand(
		newChanBackupCommand(),
		newClosePoolAccountCommand(),
		newCompactDBCommand(),
		newDeletePaymentsCommand(),
		newDeriveKeyCommand(),
		newDoubleSpendInputsCommand(),
		newDropChannelGraphCommand(),
		newDumpBackupCommand(),
		newDumpChannelsCommand(),
		newDocCommand(),
		newFakeChanBackupCommand(),
		newFilterBackupCommand(),
		newFixOldBackupCommand(),
		newForceCloseCommand(),
		newGenImportScriptCommand(),
		newMigrateDBCommand(),
		newRecoverLoopInCommand(),
		newRemoveChannelCommand(),
		newRescueClosedCommand(),
		newRescueFundingCommand(),
		newRescueTweakedKeyCommand(),
		newShowRootKeyCommand(),
		newSignRescueFundingCommand(),
		newSummaryCommand(),
		newSweepTimeLockCommand(),
		newSweepTimeLockManualCommand(),
		newSweepRemoteClosedCommand(),
		newTriggerForceCloseCommand(),
		newVanityGenCommand(),
		newWalletInfoCommand(),
		newZombieRecoveryCommand(),
	)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type rootKey struct {
	RootKey string
	BIP39   bool
}

func newRootKey(cmd *cobra.Command, desc string) *rootKey {
	r := &rootKey{}
	cmd.Flags().StringVar(
		&r.RootKey, "rootkey", "", "BIP32 HD root key of the wallet "+
			"to use for "+desc+"; leave empty to prompt for "+
			"lnd 24 word aezeed",
	)
	cmd.Flags().BoolVar(
		&r.BIP39, "bip39", false, "read a classic BIP39 seed and "+
			"passphrase from the terminal instead of asking for "+
			"lnd seed format or providing the --rootkey flag",
	)

	return r
}

func (r *rootKey) read() (*hdkeychain.ExtendedKey, error) {
	extendedKey, _, err := r.readWithBirthday()
	return extendedKey, err
}

func (r *rootKey) readWithBirthday() (*hdkeychain.ExtendedKey, time.Time,
	error) {

	// Check that root key is valid or fall back to console input.
	switch {
	case r.RootKey != "":
		extendedKey, err := hdkeychain.NewKeyFromString(r.RootKey)
		return extendedKey, time.Unix(0, 0), err

	case r.BIP39:
		extendedKey, err := btc.ReadMnemonicFromTerminal(chainParams)
		return extendedKey, time.Unix(0, 0), err

	default:
		return lnd.ReadAezeed(chainParams)
	}
}

type inputFlags struct {
	ListChannels    string
	PendingChannels string
	FromSummary     string
	FromChannelDB   string
}

func newInputFlags(cmd *cobra.Command) *inputFlags {
	f := &inputFlags{}
	cmd.Flags().StringVar(&f.ListChannels, "listchannels", "", "channel "+
		"input is in the format of lncli's listchannels format; "+
		"specify '-' to read from stdin",
	)
	cmd.Flags().StringVar(&f.PendingChannels, "pendingchannels", "", ""+
		"channel input is in the format of lncli's pendingchannels "+
		"format; specify '-' to read from stdin",
	)
	cmd.Flags().StringVar(&f.FromSummary, "fromsummary", "", "channel "+
		"input is in the format of chantool's channel summary; "+
		"specify '-' to read from stdin",
	)
	cmd.Flags().StringVar(&f.FromChannelDB, "fromchanneldb", "", "channel "+
		"input is in the format of an lnd channel.db file",
	)

	return f
}

func (f *inputFlags) parseInputType() ([]*dataformat.SummaryEntry, error) {
	var (
		content []byte
		err     error
		target  dataformat.InputFile
	)

	switch {
	case f.ListChannels != "":
		content, err = readInput(f.ListChannels)
		target = &dataformat.ListChannelsFile{}

	case f.PendingChannels != "":
		content, err = readInput(f.PendingChannels)
		target = &dataformat.PendingChannelsFile{}

	case f.FromSummary != "":
		content, err = readInput(f.FromSummary)
		target = &dataformat.SummaryEntryFile{}

	case f.FromChannelDB != "":
		db, err := lnd.OpenDB(f.FromChannelDB, true)
		if err != nil {
			return nil, fmt.Errorf("error opening channel DB: %w",
				err)
		}
		target = &dataformat.ChannelDBFile{DB: db.ChannelStateDB()}
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
	if terminal.IsTerminal(int(syscall.Stdin)) { //nolint
		fmt.Print(userQuery)
		pw, err := terminal.ReadPassword(int(syscall.Stdin)) //nolint
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

func setupLogging() {
	setSubLogger("CHAN", log)
	addSubLogger("CHDB", channeldb.UseLogger)
	addSubLogger("BCKP", chanbackup.UseLogger)
	addSubLogger("PEER", peer.UseLogger)
	err := logWriter.InitLogRotator("./results/chantools.log", 10, 3)
	if err != nil {
		panic(err)
	}
	err = build.ParseAndSetDebugLevels("debug", logWriter)
	if err != nil {
		panic(err)
	}
}

// genSubLogger creates a sub logger with an empty shutdown function.
func genSubLogger(logWriter *build.RotatingLogWriter) func(string) btclog.Logger {
	return func(s string) btclog.Logger {
		return logWriter.GenSubLogger(s, func() {})
	}
}

// addSubLogger is a helper method to conveniently create and register the
// logger of one or more sub systems.
func addSubLogger(subsystem string, useLoggers ...func(btclog.Logger)) {
	// Create and register just a single logger to prevent them from
	// overwriting each other internally.
	logger := build.NewSubLogger(subsystem, genSubLogger(logWriter))
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
