package main

import (
	"fmt"

	"github.com/coreos/bbolt"
)

const (
	dbFilePermission = 0600
	defaultTxMaxSize = 65536
)

type compactDBCommand struct {
	TxMaxSize int64  `long:"txmaxsize" description:"Maximum transaction size. (default 65536)"`
	SourceDB  string `long:"sourcedb" description:"The lnd channel.db file to create the database backup from."`
	DestDB    string `long:"destdb" description:"The lnd new channel.db file to copy the compacted database to."`
}

func (c *compactDBCommand) Execute(_ []string) error {
	// Check that we have a source and destination channel DB.
	if c.SourceDB == "" {
		return fmt.Errorf("source channel DB is required")
	}
	if c.DestDB == "" {
		return fmt.Errorf("destination channel DB is required")
	}
	if c.TxMaxSize <= 0 {
		c.TxMaxSize = defaultTxMaxSize
	}
	src, err := c.openDB(c.SourceDB, true)
	if err != nil {
		return fmt.Errorf("error opening source DB: %v", err)
	}
	dst, err := c.openDB(c.DestDB, false)
	if err != nil {
		return fmt.Errorf("error opening destination DB: %v", err)
	}
	err = c.compact(dst, src)
	if err != nil {
		return fmt.Errorf("error compacting DB: %v", err)
	}
	return nil
}

func (c *compactDBCommand) openDB(path string, ro bool) (*bbolt.DB, error) {
	options := &bbolt.Options{
		NoFreelistSync: false,
		FreelistType:   bbolt.FreelistMapType,
		ReadOnly:       ro,
	}

	bdb, err := bbolt.Open(path, dbFilePermission, options)
	if err != nil {
		return nil, err
	}
	return bdb, nil
}

func (c *compactDBCommand) compact(dst, src *bbolt.DB) error {
	// commit regularly, or we'll run out of memory for large datasets if
	// using one transaction.
	var size int64
	tx, err := dst.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := c.walk(src, func(keys [][]byte, k, v []byte, seq uint64) error {
		// On each key/value, check if we have exceeded tx size.
		sz := int64(len(k) + len(v))
		if size+sz > c.TxMaxSize && c.TxMaxSize != 0 {
			// Commit previous transaction.
			if err := tx.Commit(); err != nil {
				return err
			}

			// Start new transaction.
			tx, err = dst.Begin(true)
			if err != nil {
				return err
			}
			size = 0
		}
		size += sz

		// Create bucket on the root transaction if this is the first
		// level.
		nk := len(keys)
		if nk == 0 {
			bkt, err := tx.CreateBucket(k)
			if err != nil {
				return err
			}
			if err := bkt.SetSequence(seq); err != nil {
				return err
			}
			return nil
		}

		// Create buckets on subsequent levels, if necessary.
		b := tx.Bucket(keys[0])
		if nk > 1 {
			for _, k := range keys[1:] {
				b = b.Bucket(k)
			}
		}

		// Fill the entire page for best compaction.
		b.FillPercent = 1.0

		// If there is no value then this is a bucket call.
		if v == nil {
			bkt, err := b.CreateBucket(k)
			if err != nil {
				return err
			}
			if err := bkt.SetSequence(seq); err != nil {
				return err
			}
			return nil
		}

		// Otherwise treat it as a key/value pair.
		return b.Put(k, v)
	}); err != nil {
		return err
	}

	return tx.Commit()
}

// walkFunc is the type of the function called for keys (buckets and "normal"
// values) discovered by Walk. keys is the list of keys to descend to the bucket
// owning the discovered key/value pair k/v.
type walkFunc func(keys [][]byte, k, v []byte, seq uint64) error

// walk walks recursively the bolt database db, calling walkFn for each key it
// finds.
func (c *compactDBCommand) walk(db *bbolt.DB, walkFn walkFunc) error {
	return db.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			return c.walkBucket(
				b, nil, name, nil, b.Sequence(), walkFn,
			)
		})
	})
}

func (c *compactDBCommand) walkBucket(b *bbolt.Bucket, keypath [][]byte,
	k, v []byte, seq uint64, fn walkFunc) error {
	// Execute callback.
	if err := fn(keypath, k, v, seq); err != nil {
		return err
	}

	// If this is not a bucket then stop.
	if v != nil {
		return nil
	}

	// Iterate over each child key/value.
	keypath = append(keypath, k)
	return b.ForEach(func(k, v []byte) error {
		if v == nil {
			bkt := b.Bucket(k)
			return c.walkBucket(
				bkt, keypath, k, nil, bkt.Sequence(), fn,
			)
		}
		return c.walkBucket(b, keypath, k, v, b.Sequence(), fn)
	})
}
