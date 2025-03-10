package lnd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/btcsuite/btcwallet/walletdb"
	"github.com/lightningnetwork/lnd/channeldb"
	graphdb "github.com/lightningnetwork/lnd/graph/db"
	"github.com/lightningnetwork/lnd/kvdb"
	"go.etcd.io/bbolt"
)

const (
	DefaultOpenTimeout = time.Second * 10
)

func OpenDB(dbPath string,
	readonly bool) (*channeldb.DB, *graphdb.ChannelGraph, error) {

	backend, err := openDB(dbPath, false, readonly, DefaultOpenTimeout)
	if errors.Is(err, bbolt.ErrTimeout) {
		return nil, nil, fmt.Errorf("error opening %s: make sure lnd "+
			"is not running, database is locked by another process",
			dbPath)
	}
	if err != nil {
		return nil, nil, err
	}

	channelDB, err := channeldb.CreateWithBackend(
		backend, channeldb.OptionNoMigration(readonly),
	)
	if err != nil {
		return nil, nil, err
	}

	graphDB, err := graphdb.NewChannelGraph(
		backend, func(o *graphdb.Options) {
			o.NoMigration = readonly
		},
	)
	if err != nil {
		_ = channelDB.Close()

		return nil, nil, err
	}

	return channelDB, graphDB, nil
}

// convertErr converts some bolt errors to the equivalent walletdb error.
func convertErr(err error) error {
	switch {
	// Database open/create errors.
	case errors.Is(err, bbolt.ErrDatabaseNotOpen):
		return walletdb.ErrDbNotOpen
	case errors.Is(err, bbolt.ErrInvalid):
		return walletdb.ErrInvalid

	// Transaction errors.
	case errors.Is(err, bbolt.ErrTxNotWritable):
		return walletdb.ErrTxNotWritable
	case errors.Is(err, bbolt.ErrTxClosed):
		return walletdb.ErrTxClosed

	// Value/bucket errors.
	case errors.Is(err, bbolt.ErrBucketNotFound):
		return walletdb.ErrBucketNotFound
	case errors.Is(err, bbolt.ErrBucketExists):
		return walletdb.ErrBucketExists
	case errors.Is(err, bbolt.ErrBucketNameRequired):
		return walletdb.ErrBucketNameRequired
	case errors.Is(err, bbolt.ErrKeyRequired):
		return walletdb.ErrKeyRequired
	case errors.Is(err, bbolt.ErrKeyTooLarge):
		return walletdb.ErrKeyTooLarge
	case errors.Is(err, bbolt.ErrValueTooLarge):
		return walletdb.ErrValueTooLarge
	case errors.Is(err, bbolt.ErrIncompatibleValue):
		return walletdb.ErrIncompatibleValue
	}

	// Return the original error if none of the above applies.
	return err
}

// transaction represents a database transaction.  It can either by read-only or
// read-write and implements the walletdb Tx interfaces.  The transaction
// provides a root bucket against which all read and writes occur.
type transaction struct {
	boltTx *bbolt.Tx
}

func (tx *transaction) ReadBucket(key []byte) walletdb.ReadBucket {
	return tx.ReadWriteBucket(key)
}

// ForEachBucket will iterate through all top level buckets.
func (tx *transaction) ForEachBucket(fn func(key []byte) error) error {
	return convertErr(tx.boltTx.ForEach(
		func(name []byte, _ *bbolt.Bucket) error {
			return fn(name)
		},
	))
}

func (tx *transaction) ReadWriteBucket(key []byte) walletdb.ReadWriteBucket {
	boltBucket := tx.boltTx.Bucket(key)
	if boltBucket == nil {
		return nil
	}
	return (*bucket)(boltBucket)
}

func (tx *transaction) CreateTopLevelBucket(key []byte) (walletdb.ReadWriteBucket, error) {
	boltBucket, err := tx.boltTx.CreateBucketIfNotExists(key)
	if err != nil {
		return nil, convertErr(err)
	}
	return (*bucket)(boltBucket), nil
}

func (tx *transaction) DeleteTopLevelBucket(key []byte) error {
	err := tx.boltTx.DeleteBucket(key)
	if err != nil {
		return convertErr(err)
	}
	return nil
}

// Commit commits all changes that have been made through the root bucket and
// all of its sub-buckets to persistent storage.
//
// This function is part of the walletdb.ReadWriteTx interface implementation.
func (tx *transaction) Commit() error {
	return convertErr(tx.boltTx.Commit())
}

// Rollback undoes all changes that have been made to the root bucket and all of
// its sub-buckets.
//
// This function is part of the walletdb.ReadTx interface implementation.
func (tx *transaction) Rollback() error {
	return convertErr(tx.boltTx.Rollback())
}

// OnCommit takes a function closure that will be executed when the transaction
// successfully gets committed.
//
// This function is part of the walletdb.ReadWriteTx interface implementation.
func (tx *transaction) OnCommit(f func()) {
	tx.boltTx.OnCommit(f)
}

// bucket is an internal type used to represent a collection of key/value pairs
// and implements the walletdb Bucket interfaces.
type bucket bbolt.Bucket

// Enforce bucket implements the walletdb Bucket interfaces.
var _ walletdb.ReadWriteBucket = (*bucket)(nil)

// NestedReadWriteBucket retrieves a nested bucket with the given key.  Returns
// nil if the bucket does not exist.
//
// This function is part of the walletdb.ReadWriteBucket interface implementation.
func (b *bucket) NestedReadWriteBucket(key []byte) walletdb.ReadWriteBucket {
	boltBucket := (*bbolt.Bucket)(b).Bucket(key)
	// Don't return a non-nil interface to a nil pointer.
	if boltBucket == nil {
		return nil
	}
	return (*bucket)(boltBucket)
}

func (b *bucket) NestedReadBucket(key []byte) walletdb.ReadBucket {
	return b.NestedReadWriteBucket(key)
}

// CreateBucket creates and returns a new nested bucket with the given key.
// Returns ErrBucketExists if the bucket already exists, ErrBucketNameRequired
// if the key is empty, or ErrIncompatibleValue if the key value is otherwise
// invalid.
//
// This function is part of the walletdb.ReadWriteBucket interface implementation.
func (b *bucket) CreateBucket(key []byte) (walletdb.ReadWriteBucket, error) {
	boltBucket, err := (*bbolt.Bucket)(b).CreateBucket(key)
	if err != nil {
		return nil, convertErr(err)
	}
	return (*bucket)(boltBucket), nil
}

// CreateBucketIfNotExists creates and returns a new nested bucket with the
// given key if it does not already exist.  Returns ErrBucketNameRequired if the
// key is empty or ErrIncompatibleValue if the key value is otherwise invalid.
//
// This function is part of the walletdb.ReadWriteBucket interface implementation.
func (b *bucket) CreateBucketIfNotExists(key []byte) (walletdb.ReadWriteBucket, error) {
	boltBucket, err := (*bbolt.Bucket)(b).CreateBucketIfNotExists(key)
	if err != nil {
		return nil, convertErr(err)
	}
	return (*bucket)(boltBucket), nil
}

// DeleteNestedBucket removes a nested bucket with the given key.  Returns
// ErrTxNotWritable if attempted against a read-only transaction and
// ErrBucketNotFound if the specified bucket does not exist.
//
// This function is part of the walletdb.ReadWriteBucket interface implementation.
func (b *bucket) DeleteNestedBucket(key []byte) error {
	return convertErr((*bbolt.Bucket)(b).DeleteBucket(key))
}

// ForEach invokes the passed function with every key/value pair in the bucket.
// This includes nested buckets, in which case the value is nil, but it does not
// include the key/value pairs within those nested buckets.
//
// NOTE: The values returned by this function are only valid during a
// transaction.  Attempting to access them after a transaction has ended will
// likely result in an access violation.
//
// This function is part of the walletdb.ReadBucket interface implementation.
func (b *bucket) ForEach(fn func(k, v []byte) error) error {
	return convertErr((*bbolt.Bucket)(b).ForEach(fn))
}

// Put saves the specified key/value pair to the bucket.  Keys that do not
// already exist are added and keys that already exist are overwritten.  Returns
// ErrTxNotWritable if attempted against a read-only transaction.
//
// This function is part of the walletdb.ReadWriteBucket interface implementation.
func (b *bucket) Put(key, value []byte) error {
	return convertErr((*bbolt.Bucket)(b).Put(key, value))
}

// Get returns the value for the given key.  Returns nil if the key does
// not exist in this bucket (or nested buckets).
//
// NOTE: The value returned by this function is only valid during a
// transaction.  Attempting to access it after a transaction has ended
// will likely result in an access violation.
//
// This function is part of the walletdb.ReadBucket interface implementation.
func (b *bucket) Get(key []byte) []byte {
	return (*bbolt.Bucket)(b).Get(key)
}

// Delete removes the specified key from the bucket.  Deleting a key that does
// not exist does not return an error.  Returns ErrTxNotWritable if attempted
// against a read-only transaction.
//
// This function is part of the walletdb.ReadWriteBucket interface implementation.
func (b *bucket) Delete(key []byte) error {
	return convertErr((*bbolt.Bucket)(b).Delete(key))
}

func (b *bucket) ReadCursor() walletdb.ReadCursor {
	return b.ReadWriteCursor()
}

// ReadWriteCursor returns a new cursor, allowing for iteration over the bucket's
// key/value pairs and nested buckets in forward or backward order.
//
// This function is part of the walletdb.ReadWriteBucket interface implementation.
func (b *bucket) ReadWriteCursor() walletdb.ReadWriteCursor {
	return (*cursor)((*bbolt.Bucket)(b).Cursor())
}

// Tx returns the bucket's transaction.
//
// This function is part of the walletdb.ReadWriteBucket interface implementation.
func (b *bucket) Tx() walletdb.ReadWriteTx {
	return &transaction{
		(*bbolt.Bucket)(b).Tx(),
	}
}

// NextSequence returns an autoincrementing integer for the bucket.
func (b *bucket) NextSequence() (uint64, error) {
	return (*bbolt.Bucket)(b).NextSequence()
}

// SetSequence updates the sequence number for the bucket.
func (b *bucket) SetSequence(v uint64) error {
	return (*bbolt.Bucket)(b).SetSequence(v)
}

// Sequence returns the current integer for the bucket without incrementing it.
func (b *bucket) Sequence() uint64 {
	return (*bbolt.Bucket)(b).Sequence()
}

// cursor represents a cursor over key/value pairs and nested buckets of a
// bucket.
//
// Note that open cursors are not tracked on bucket changes and any
// modifications to the bucket, with the exception of cursor.Delete, invalidate
// the cursor. After invalidation, the cursor must be repositioned, or the keys
// and values returned may be unpredictable.
type cursor bbolt.Cursor

// Delete removes the current key/value pair the cursor is at without
// invalidating the cursor. Returns ErrTxNotWritable if attempted on a read-only
// transaction, or ErrIncompatibleValue if attempted when the cursor points to a
// nested bucket.
//
// This function is part of the walletdb.ReadWriteCursor interface implementation.
func (c *cursor) Delete() error {
	return convertErr((*bbolt.Cursor)(c).Delete())
}

// First positions the cursor at the first key/value pair and returns the pair.
//
// This function is part of the walletdb.ReadCursor interface implementation.
func (c *cursor) First() ([]byte, []byte) {
	return (*bbolt.Cursor)(c).First()
}

// Last positions the cursor at the last key/value pair and returns the pair.
//
// This function is part of the walletdb.ReadCursor interface implementation.
func (c *cursor) Last() ([]byte, []byte) {
	return (*bbolt.Cursor)(c).Last()
}

// Next moves the cursor one key/value pair forward and returns the new pair.
//
// This function is part of the walletdb.ReadCursor interface implementation.
func (c *cursor) Next() ([]byte, []byte) {
	return (*bbolt.Cursor)(c).Next()
}

// Prev moves the cursor one key/value pair backward and returns the new pair.
//
// This function is part of the walletdb.ReadCursor interface implementation.
func (c *cursor) Prev() ([]byte, []byte) {
	return (*bbolt.Cursor)(c).Prev()
}

// Seek positions the cursor at the passed seek key. If the key does not exist,
// the cursor is moved to the next key after seek. Returns the new pair.
//
// This function is part of the walletdb.ReadCursor interface implementation.
func (c *cursor) Seek(seek []byte) ([]byte, []byte) {
	return (*bbolt.Cursor)(c).Seek(seek)
}

// backend represents a collection of namespaces which are persisted and
// implements the kvdb.Backend interface. All database access is performed
// through transactions which are obtained through the specific Namespace.
type backend struct {
	db *bbolt.DB
}

// Enforce db implements the kvdb.Backend interface.
var _ kvdb.Backend = (*backend)(nil)

func (b *backend) beginTx(writable bool) (*transaction, error) {
	boltTx, err := b.db.Begin(writable)
	if err != nil {
		return nil, convertErr(err)
	}
	return &transaction{boltTx: boltTx}, nil
}

func (b *backend) BeginReadTx() (walletdb.ReadTx, error) {
	return b.beginTx(false)
}

func (b *backend) BeginReadWriteTx() (walletdb.ReadWriteTx, error) {
	return b.beginTx(true)
}

// Copy writes a copy of the database to the provided writer.  This call will
// start a read-only transaction to perform all operations.
//
// This function is part of the kvdb.Backend interface implementation.
func (b *backend) Copy(w io.Writer) error {
	return convertErr(b.db.View(func(tx *bbolt.Tx) error {
		return tx.Copy(w)
	}))
}

// Close cleanly shuts down the database and syncs all data.
//
// This function is part of the kvdb.Backend interface implementation.
func (b *backend) Close() error {
	return convertErr(b.db.Close())
}

// Batch is similar to the package-level Update method, but it will attempt to
// optimistically combine the invocation of several transaction functions into a
// single db write transaction.
//
// This function is part of the walletdb.Db interface implementation.
func (b *backend) Batch(f func(tx walletdb.ReadWriteTx) error) error {
	return b.db.Batch(func(btx *bbolt.Tx) error {
		interfaceTx := transaction{btx}

		return f(&interfaceTx)
	})
}

// View opens a database read transaction and executes the function f with the
// transaction passed as a parameter. After f exits, the transaction is rolled
// back. If f errors, its error is returned, not a rollback error (if any
// occur). The passed reset function is called before the start of the
// transaction and can be used to reset intermediate state. As callers may
// expect retries of the f closure (depending on the database backend used), the
// reset function will be called before each retry respectively.
func (b *backend) View(f func(tx walletdb.ReadTx) error, reset func()) error {
	// We don't do any retries with bolt so we just initially call the reset
	// function once.
	reset()

	tx, err := b.BeginReadTx()
	if err != nil {
		return err
	}

	// Make sure the transaction rolls back in the event of a panic.
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	err = f(tx)
	rollbackErr := tx.Rollback()
	if err != nil {
		return err
	}

	if rollbackErr != nil {
		return rollbackErr
	}
	return nil
}

// Update opens a database read/write transaction and executes the function f
// with the transaction passed as a parameter. After f exits, if f did not
// error, the transaction is committed. Otherwise, if f did error, the
// transaction is rolled back. If the rollback fails, the original error
// returned by f is still returned. If the commit fails, the commit error is
// returned. As callers may expect retries of the f closure (depending on the
// database backend used), the reset function will be called before each retry
// respectively.
func (b *backend) Update(f func(tx walletdb.ReadWriteTx) error,
	reset func()) error {

	// We don't do any retries with bolt so we just initially call the reset
	// function once.
	reset()

	tx, err := b.BeginReadWriteTx()
	if err != nil {
		return err
	}

	// Make sure the transaction rolls back in the event of a panic.
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	err = f(tx)
	if err != nil {
		// Want to return the original error, not a rollback error if
		// any occur.
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// PrintStats returns all collected stats pretty printed into a string.
func (b *backend) PrintStats() string {
	return "n/a"
}

// filesExists reports whether the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// openDB opens the database at the provided path. walletdb.ErrDbDoesNotExist
// is returned if the database doesn't exist and the create flag is not set.
func openDB(dbPath string, noFreelistSync bool,
	readonly bool, timeout time.Duration) (kvdb.Backend, error) {

	if !fileExists(dbPath) {
		return nil, walletdb.ErrDbDoesNotExist
	}

	// Specify bbolt freelist options to reduce heap pressure in case the
	// freelist grows to be very large.
	options := &bbolt.Options{
		NoFreelistSync: noFreelistSync,
		FreelistType:   bbolt.FreelistMapType,
		Timeout:        timeout,
		ReadOnly:       readonly,
	}

	boltDB, err := bbolt.Open(dbPath, 0600, options)
	return &backend{db: boltDB}, convertErr(err)
}
