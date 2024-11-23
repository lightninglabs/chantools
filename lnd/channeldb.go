package lnd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/kvdb"
	"github.com/lightningnetwork/lnd/kvdb/etcd"
	"github.com/lightningnetwork/lnd/kvdb/postgres"
	"github.com/lightningnetwork/lnd/kvdb/sqlbase"
	"github.com/lightningnetwork/lnd/kvdb/sqlite"
	"github.com/lightningnetwork/lnd/lncfg"
	"go.etcd.io/bbolt"
)

// Bolt specifies the settings for the bolt database.
type Bolt struct {
	DBTimeout time.Duration
	DataDir   string
	TowerDir  string
	Name      string
}

// Sqlite specifies the settings for the sqlite database.
type Sqlite struct {
	DataDir  string
	TowerDir string
	Config   *sqlite.Config
}

// DB specifies the settings for all different database backends.
type DB struct {
	Backend  string
	Etcd     *etcd.Config
	Bolt     *Bolt
	Postgres *postgres.Config
	Sqlite   *Sqlite
}

const (
	// DefaultOpenTimeout is the default timeout for opening a database.
	DefaultOpenTimeout = time.Second * 10
)

// Init should be called upon start to pre-initialize database for sql
// backends. If max connections are not set, the amount of connections will be
// unlimited however we only use one connection during the migration.
func (db DB) Init() error {
	// Start embedded etcd server if requested.
	switch {
	case db.Backend == lncfg.PostgresBackend:
		sqlbase.Init(db.Postgres.MaxConnections)

	case db.Backend == lncfg.SqliteBackend:
		sqlbase.Init(db.Sqlite.Config.MaxConnections)
	}

	return nil
}

// DBOption is a functional option for configuring the database.
type DBOption func(*dbOptions)

type dbOptions struct {
	customGraphDir  string
	customWalletDir string
}

// WithCustomGraphDir sets a custom directory for the graph database.
func WithCustomGraphDir(dir string) DBOption {
	return func(opts *dbOptions) {
		opts.customGraphDir = dir
	}
}

// WithCustomWalletDir sets a custom directory for the wallet database.
func WithCustomWalletDir(dir string) DBOption {
	return func(opts *dbOptions) {
		opts.customWalletDir = dir
	}
}

func openDB(cfg DB, prefix, network string,
	opts ...DBOption) (kvdb.Backend, error) {

	backend := cfg.Backend

	// Init the db connections for sql backends.
	err := cfg.Init()
	if err != nil {
		return nil, err
	}

	options := &dbOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Settings to open a particular db backend.
	var args []interface{}

	switch backend {
	case lncfg.BoltBackend:
		// Directories where the db files are located.
		var graphDir string
		if options.customGraphDir != "" {
			graphDir = options.customGraphDir
		} else {
			graphDir = filepath.Join(
				cfg.Bolt.DataDir, "graph", network,
			)
		}

		var walletDir string
		if options.customWalletDir != "" {
			walletDir = options.customWalletDir
		} else {
			walletDir = filepath.Join(
				cfg.Bolt.DataDir, "chain", "bitcoin", network,
			)
		}

		towerServerDir := filepath.Join(
			cfg.Bolt.TowerDir, "bitcoin", network,
		)

		// Path to the db file.
		var path string
		switch prefix {
		case lncfg.NSChannelDB:
			if cfg.Bolt.Name != "" {
				path = filepath.Join(graphDir, cfg.Bolt.Name)
			} else {
				path = filepath.Join(graphDir, lncfg.ChannelDBName)
			}

		case lncfg.NSMacaroonDB:
			path = filepath.Join(walletDir, lncfg.MacaroonDBName)

		case lncfg.NSDecayedLogDB:
			path = filepath.Join(graphDir, lncfg.DecayedLogDbName)

		case lncfg.NSTowerClientDB:
			path = filepath.Join(graphDir, lncfg.TowerClientDBName)

		case lncfg.NSTowerServerDB:
			path = filepath.Join(
				towerServerDir, lncfg.TowerServerDBName,
			)

		case lncfg.NSWalletDB:
			path = filepath.Join(walletDir, lncfg.WalletDBName)
		}

		const (
			noFreelistSync = false
			timeout        = time.Minute
		)

		args = []interface{}{
			path, noFreelistSync, timeout,
		}
		backend = kvdb.BoltBackendName

	case kvdb.EtcdBackendName:
		args = []interface{}{
			context.Background(),
			cfg.Etcd.CloneWithSubNamespace(prefix),
		}

	case kvdb.PostgresBackendName:
		args = []interface{}{
			context.Background(),
			&postgres.Config{
				Dsn:            cfg.Postgres.Dsn,
				Timeout:        time.Minute,
				MaxConnections: 10,
			},
			prefix,
		}

	case kvdb.SqliteBackendName:
		// Directories where the db files are located.
		var graphDir string
		if options.customGraphDir != "" {
			graphDir = options.customGraphDir
		} else {
			graphDir = filepath.Join(
				cfg.Sqlite.DataDir, "graph", network,
			)
		}

		var walletDir string
		if options.customWalletDir != "" {
			walletDir = options.customWalletDir
		} else {
			walletDir = filepath.Join(
				cfg.Sqlite.DataDir, "chain", "bitcoin", network,
			)
		}

		towerServerDir := filepath.Join(
			cfg.Sqlite.TowerDir, "bitcoin", network,
		)

		var dbName string
		var path string
		switch prefix {
		case lncfg.NSChannelDB:
			path = graphDir
			dbName = lncfg.SqliteChannelDBName
		case lncfg.NSMacaroonDB:
			path = walletDir
			dbName = lncfg.SqliteChainDBName

		case lncfg.NSDecayedLogDB:
			path = graphDir
			dbName = lncfg.SqliteChannelDBName

		case lncfg.NSTowerClientDB:
			path = graphDir
			dbName = lncfg.SqliteChannelDBName

		case lncfg.NSTowerServerDB:
			path = towerServerDir
			dbName = lncfg.SqliteChannelDBName

		case lncfg.NSWalletDB:
			path = walletDir
			dbName = lncfg.SqliteChainDBName

		case lncfg.NSNeutrinoDB:
			dbName = lncfg.SqliteNeutrinoDBName
		}

		args = []interface{}{
			context.Background(),
			&sqlite.Config{
				Timeout: time.Minute,
			},
			path,
			dbName,
			prefix,
		}

	default:
		return nil, fmt.Errorf("unknown backend: %v", backend)
	}

	return kvdb.Open(backend, args...)
}

// OpenChannelDB opens the channel database for all supported backends.
func OpenChannelDB(cfg DB, readonly bool, network string,
	opts ...DBOption) (*channeldb.DB, error) {

	backend, err := openDB(cfg, lncfg.NSChannelDB, network, opts...)

	// For the bbolt backend, we signal a detailed error message if the
	// database is locked by another process.
	if errors.Is(err, bbolt.ErrTimeout) {
		return nil, errors.New("error opening boltDB: make sure lnd " +
			"is not running, database is locked by another process")
	}
	if err != nil {
		return nil, err
	}

	return channeldb.CreateWithBackend(
		backend, channeldb.OptionSetUseGraphCache(false),
		channeldb.OptionNoMigration(readonly),
	)
}
