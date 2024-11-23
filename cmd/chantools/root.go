package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btclog"
	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/dataformat"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/build"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/kvdb/postgres"
	"github.com/lightningnetwork/lnd/kvdb/sqlite"
	"github.com/lightningnetwork/lnd/peer"
	"github.com/ory/viper"
	"github.com/spf13/cobra"
)

const (
	defaultAPIURL        = "https://api.node-recovery.com"
	defaultTestnetAPIURL = "https://blockstream.info/testnet/api"
	defaultRegtestAPIURL = "http://localhost:3004"

	// version is the current version of the tool. It is set during build.
	// NOTE: When changing this, please also update the version in the
	// download link shown in the README.
	version = "0.13.4"
	na      = "n/a"

	// lndVersion is the current version of lnd that we support. This is
	// shown in some commands that affect the database and its migrations.
	// Run "make docs" after changing this value.
	lndVersion = "v0.18.3-beta"

	Commit = ""
)

var (
	Testnet bool
	Regtest bool
	Signet  bool

	logWriter   = build.NewRotatingLogWriter()
	log         = build.NewSubLogger("CHAN", genSubLogger(logWriter))
	chainParams = &chaincfg.MainNetParams

	// defaultDataDir is the default data directory for lnd.
	defaultDataDir = filepath.Join(btcutil.AppDataDir("lnd", false), "data")
)

var rootCmd = &cobra.Command{
	Use:   "chantools",
	Short: "Chantools helps recover funds from lightning channels",
	Long: `This tool provides helper functions that can be used rescue
funds locked in lnd channels in case lnd itself cannot run properly anymore.
Complete documentation is available at
https://github.com/lightninglabs/chantools/.`,
	Version: fmt.Sprintf("v%s, commit %s", version, Commit),
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		switch {
		case Testnet:
			chainParams = &chaincfg.TestNet3Params

		case Regtest:
			chainParams = &chaincfg.RegressionNetParams

		case Signet:
			chainParams = &chaincfg.SigNetParams

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
	rootCmd.PersistentFlags().BoolVarP(
		&Signet, "signet", "s", false, "Indicates if the public "+
			"signet parameters should be used",
	)
	rootCmd.PersistentFlags().String("db.backend", "bolt", "The selected "+
		"database backend (bolt/etcd/postgres/sqlite)",
	)

	// Bolt settings
	rootCmd.PersistentFlags().Duration("db.bolt.dbtimeout", 10*time.Second,
		"Specify the timeout value used when opening the database",
	)
	rootCmd.PersistentFlags().String("db.bolt.data-dir", defaultDataDir,
		"Lnd data dir where bolt dbs are located",
	)
	rootCmd.PersistentFlags().String("db.bolt.tower-dir", defaultDataDir,
		"Lnd watchtower dir where bolt dbs are located",
	)
	rootCmd.PersistentFlags().String("db.bolt.name", "", "Name of the bolt"+
		"db to use",
	)

	// Sqlite settings
	rootCmd.PersistentFlags().String("db.sqlite.data-dir", defaultDataDir,
		"Lnd data dir where sqlite dbs are located",
	)
	rootCmd.PersistentFlags().String("db.sqlite.tower-dir", defaultDataDir,
		"Lnd watchtower dir where sqlite dbs are located",
	)
	rootCmd.PersistentFlags().Duration("db.sqlite.timeout", 10*time.Second,
		"Specify the timeout value used when opening the database",
	)

	// Postgres settings
	rootCmd.PersistentFlags().String("db.postgres.dsn", "", "Postgres "+
		"connection string",
	)
	rootCmd.PersistentFlags().Duration("db.postgres.timeout",
		10*time.Second, "Specify the timeout value used when opening"+
			"the database",
	)

	// Bind flags to viper
	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		log.Errorf("error binding flags: %v", err)
	}

	rootCmd.AddCommand(
		newChanBackupCommand(),
		newClosePoolAccountCommand(),
		newCreateWalletCommand(),
		newCompactDBCommand(),
		newDeletePaymentsCommand(),
		newDeriveKeyCommand(),
		newDoubleSpendInputsCommand(),
		newDropChannelGraphCommand(),
		newDropGraphZombiesCommand(),
		newDumpBackupCommand(),
		newDumpChannelsCommand(),
		newDocCommand(),
		newFakeChanBackupCommand(),
		newFilterBackupCommand(),
		newFixOldBackupCommand(),
		newForceCloseCommand(),
		newGenImportScriptCommand(),
		newMigrateDBCommand(),
		newPullAnchorCommand(),
		newRecoverLoopInCommand(),
		newRemoveChannelCommand(),
		newRescueClosedCommand(),
		newRescueFundingCommand(),
		newRescueTweakedKeyCommand(),
		newShowRootKeyCommand(),
		newSignMessageCommand(),
		newSignRescueFundingCommand(),
		newSignPSBTCommand(),
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
	RootKey  string
	BIP39    bool
	WalletDB string
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
	cmd.Flags().StringVar(
		&r.WalletDB, "walletdb", "", "read the seed/master root key "+
			"to use fro "+desc+" from an lnd wallet.db file "+
			"instead of asking for a seed or providing the "+
			"--rootkey flag",
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

	case r.WalletDB != "":
		cfg := GetDBConfig()
		wallet, pw, cleanup, err := lnd.OpenWallet(
			cfg, r.WalletDB, chainParams,
		)
		if err != nil {
			return nil, time.Unix(0, 0), fmt.Errorf("error "+
				"opening wallet '%s': %w", r.WalletDB, err)
		}

		defer func() {
			if err := cleanup(); err != nil {
				log.Errorf("error closing wallet: %v", err)
			}
		}()

		extendedKeyBytes, err := lnd.DecryptWalletRootKey(
			wallet.Database(), pw,
		)
		if err != nil {
			return nil, time.Unix(0, 0), fmt.Errorf("error "+
				"decrypting wallet root key: %w", err)
		}

		extendedKey, err := hdkeychain.NewKeyFromString(
			string(extendedKeyBytes),
		)
		if err != nil {
			return nil, time.Unix(0, 0), fmt.Errorf("error "+
				"parsing master key: %w", err)
		}

		return extendedKey, wallet.Manager.Birthday(), nil

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
		var opts []lnd.DBOption

		// In case the channel DB is specified, we get the graph dir
		// from it.
		if f.FromChannelDB != "" {
			graphDir := filepath.Dir(f.FromChannelDB)
			opts = append(opts, lnd.WithCustomGraphDir(graphDir))
		}

		dbConfig := GetDBConfig()

		db, err := lnd.OpenChannelDB(
			dbConfig, true, chainParams.Name, opts...,
		)
		if err != nil {
			return nil, fmt.Errorf("error opening rescue DB: %w",
				err)
		}
		target = &dataformat.ChannelDBFile{DB: db.ChannelStateDB()}
		return target.AsSummaryEntries()

	default:
		return nil, errors.New("an input file must be specified")
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
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(input)
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

func newExplorerAPI(apiURL string) *btc.ExplorerAPI {
	// Override for testnet if default is used.
	if apiURL == defaultAPIURL &&
		chainParams.Name == chaincfg.TestNet3Params.Name {

		return &btc.ExplorerAPI{BaseURL: defaultTestnetAPIURL}
	}

	// Also override for regtest if default is used.
	if apiURL == defaultAPIURL &&
		chainParams.Name == chaincfg.RegressionNetParams.Name {

		return &btc.ExplorerAPI{BaseURL: defaultRegtestAPIURL}
	}

	// Otherwise use the provided URL.
	return &btc.ExplorerAPI{BaseURL: apiURL}
}

// GetDBConfig returns the database configuration from viper/cobra flags.
func GetDBConfig() lnd.DB {
	return lnd.DB{
		Backend: viper.GetString("db.backend"),
		Bolt: &lnd.Bolt{
			DBTimeout: viper.GetDuration("db.bolt.dbtimeout"),
			DataDir:   viper.GetString("db.bolt.data-dir"),
			TowerDir:  viper.GetString("db.bolt.tower-dir"),
			Name:      viper.GetString("db.bolt.name"),
		},
		Sqlite: &lnd.Sqlite{
			DataDir:  viper.GetString("db.sqlite.data-dir"),
			TowerDir: viper.GetString("db.sqlite.tower-dir"),
			Config: &sqlite.Config{
				MaxConnections: viper.GetInt("db.sqlite.timeout"),
			},
		},
		Postgres: &postgres.Config{
			Dsn:            viper.GetString("db.postgres.dsn"),
			MaxConnections: viper.GetInt("db.postgres.max-connections"),
			Timeout:        viper.GetDuration("db.postgres.timeout"),
		},
		// Add Etcd config if needed
	}
}
