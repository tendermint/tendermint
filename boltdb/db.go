package boltdb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tmdb "github.com/tendermint/tm-db"
	"go.etcd.io/bbolt"
)

var bucket = []byte("tm")

// BoltDB is a wrapper around etcd's fork of bolt (https://github.com/etcd-io/bbolt).
//
// NOTE: All operations (including Set, Delete) are synchronous by default. One
// can globally turn it off by using NoSync config option (not recommended).
//
// A single bucket ([]byte("tm")) is used per a database instance. This could
// lead to performance issues when/if there will be lots of keys.
type BoltDB struct {
	db *bbolt.DB
}

var _ tmdb.DB = (*BoltDB)(nil)

// NewDB returns a BoltDB with default options.
func NewDB(name, dir string) (tmdb.DB, error) {
	return NewDBWithOpts(name, dir, bbolt.DefaultOptions)
}

// NewDBWithOpts allows you to supply *bbolt.Options. ReadOnly: true is not
// supported because NewDBWithOpts creates a global bucket.
func NewDBWithOpts(name string, dir string, opts *bbolt.Options) (tmdb.DB, error) {
	if opts.ReadOnly {
		return nil, errors.New("ReadOnly: true is not supported")
	}

	dbPath := filepath.Join(dir, name+".db")
	db, err := bbolt.Open(dbPath, os.ModePerm, opts)
	if err != nil {
		return nil, err
	}

	// create a global bucket
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		return err
	})
	if err != nil {
		return nil, err
	}

	return &BoltDB{db: db}, nil
}

// Get implements DB.
func (bdb *BoltDB) Get(key []byte) (value []byte, err error) {
	if len(key) == 0 {
		return nil, tmdb.ErrKeyEmpty
	}
	err = bdb.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucket)
		if v := b.Get(key); v != nil {
			value = append([]byte{}, v...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return
}

// Has implements DB.
func (bdb *BoltDB) Has(key []byte) (bool, error) {
	bytes, err := bdb.Get(key)
	if err != nil {
		return false, err
	}
	return bytes != nil, nil
}

// Set implements DB.
func (bdb *BoltDB) Set(key, value []byte) error {
	if len(key) == 0 {
		return tmdb.ErrKeyEmpty
	}
	if value == nil {
		return tmdb.ErrValueNil
	}
	err := bdb.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucket)
		return b.Put(key, value)
	})
	if err != nil {
		return err
	}
	return nil
}

// SetSync implements DB.
func (bdb *BoltDB) SetSync(key, value []byte) error {
	return bdb.Set(key, value)
}

// Delete implements DB.
func (bdb *BoltDB) Delete(key []byte) error {
	if len(key) == 0 {
		return tmdb.ErrKeyEmpty
	}
	err := bdb.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucket).Delete(key)
	})
	if err != nil {
		return err
	}
	return nil
}

// DeleteSync implements DB.
func (bdb *BoltDB) DeleteSync(key []byte) error {
	return bdb.Delete(key)
}

// Close implements DB.
func (bdb *BoltDB) Close() error {
	return bdb.db.Close()
}

// Print implements DB.
func (bdb *BoltDB) Print() error {
	stats := bdb.db.Stats()
	fmt.Printf("%v\n", stats)

	err := bdb.db.View(func(tx *bbolt.Tx) error {
		if err := tx.Bucket(bucket).ForEach(func(k, v []byte) error {
			fmt.Printf("[%X]:\t[%X]\n", k, v)
			return nil
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// Stats implements DB.
func (bdb *BoltDB) Stats() map[string]string {
	stats := bdb.db.Stats()
	m := make(map[string]string)

	// Freelist stats
	m["FreePageN"] = fmt.Sprintf("%v", stats.FreePageN)
	m["PendingPageN"] = fmt.Sprintf("%v", stats.PendingPageN)
	m["FreeAlloc"] = fmt.Sprintf("%v", stats.FreeAlloc)
	m["FreelistInuse"] = fmt.Sprintf("%v", stats.FreelistInuse)

	// Transaction stats
	m["TxN"] = fmt.Sprintf("%v", stats.TxN)
	m["OpenTxN"] = fmt.Sprintf("%v", stats.OpenTxN)

	return m
}

// NewBatch implements DB.
func (bdb *BoltDB) NewBatch() tmdb.Batch {
	return newBoltDBBatch(bdb)
}

// WARNING: Any concurrent writes or reads will block until the iterator is
// closed.
func (bdb *BoltDB) Iterator(start, end []byte) (tmdb.Iterator, error) {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		return nil, tmdb.ErrKeyEmpty
	}
	tx, err := bdb.db.Begin(false)
	if err != nil {
		return nil, err
	}
	return newBoltDBIterator(tx, start, end, false), nil
}

// WARNING: Any concurrent writes or reads will block until the iterator is
// closed.
func (bdb *BoltDB) ReverseIterator(start, end []byte) (tmdb.Iterator, error) {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		return nil, tmdb.ErrKeyEmpty
	}
	tx, err := bdb.db.Begin(false)
	if err != nil {
		return nil, err
	}
	return newBoltDBIterator(tx, start, end, true), nil
}
