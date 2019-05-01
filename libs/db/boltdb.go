package db

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/etcd-io/bbolt"
)

// A sigle bucket is used per a database instance. This could lead to
// performance issues when/if there will be lots of keys.
var bucket = []byte("tm")

func init() {
	registerDBCreator(BoltDBBackend, func(name, dir string) (DB, error) {
		return NewBoltDB(name, dir)
	}, false)
}

// BoltDB is a wrapper around etcd's fork of bolt
// (https://github.com/etcd-io/bbolt).
//
// NOTE: All operations (including Set, Delete) are synchronous by default. One
// can globally turn it off by using NoSync config option (not recommended).
type BoltDB struct {
	db *bbolt.DB
}

// NewBoltDB returns a BoltDB with default options.
func NewBoltDB(name, dir string) (DB, error) {
	return NewBoltDBWithOpts(name, dir, bbolt.DefaultOptions)
}

// NewBoltDBWithOpts allows you to supply *bbolt.Options.
func NewBoltDBWithOpts(name string, dir string, opts *bbolt.Options) (DB, error) {
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

func (bdb *BoltDB) Get(key []byte) (value []byte) {
	key = nonEmptyKey(nonNilBytes(key))
	err := bdb.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucket)
		value = b.Get(key)
		return nil
	})
	if err != nil {
		panic(err)
	}
	return
}

func (bdb *BoltDB) Has(key []byte) bool {
	return bdb.Get(key) != nil
}

func (bdb *BoltDB) Set(key, value []byte) {
	key = nonEmptyKey(nonNilBytes(key))
	value = nonNilBytes(value)
	err := bdb.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucket)
		return b.Put(key, value)
	})
	if err != nil {
		panic(err)
	}
}

func (bdb *BoltDB) SetSync(key, value []byte) {
	bdb.Set(key, value)
}

func (bdb *BoltDB) Delete(key []byte) {
	key = nonEmptyKey(nonNilBytes(key))
	err := bdb.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucket).Delete(key)
	})
	if err != nil {
		panic(err)
	}
}

func (bdb *BoltDB) DeleteSync(key []byte) {
	bdb.Delete(key)
}

func (bdb *BoltDB) Close() {
	bdb.db.Close()
}

func (bdb *BoltDB) Print() {
	stats := bdb.db.Stats()
	fmt.Printf("%v\n", stats)

	bdb.db.View(func(tx *bbolt.Tx) error {
		tx.Bucket(bucket).ForEach(func(k, v []byte) error {
			fmt.Printf("[%X]:\t[%X]\n", k, v)
			return nil
		})
		return nil
	})
}

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

// boltDBBatch stores key values in sync.Map and dumps them to the underlying
// DB upon Write call.
type boltDBBatch struct {
	buffer *sync.Map
	db     *BoltDB
}

// NewBatch returns a new batch.
func (bdb *BoltDB) NewBatch() Batch {
	return &boltDBBatch{
		buffer: &sync.Map{},
		db:     bdb,
	}
}

func (bdb *boltDBBatch) Set(key, value []byte) {
	bdb.buffer.Store(key, value)
}

func (bdb *boltDBBatch) Delete(key []byte) {
	bdb.buffer.Delete(key)
}

// NOTE: the operation is synchronous (see BoltDB for reasons)
func (bdb *boltDBBatch) Write() {
	err := bdb.db.db.Batch(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucket)
		var putErr error
		bdb.buffer.Range(func(key, value interface{}) bool {
			putErr = b.Put(key.([]byte), value.([]byte))
			return putErr == nil // stop if putErr is not nil
		})
		return putErr
	})
	if err != nil {
		panic(err)
	}
}

func (bdb *boltDBBatch) WriteSync() {
	bdb.Write()
}

func (bdb *boltDBBatch) Close() {}

func (bdb *BoltDB) Iterator(start, end []byte) Iterator {
	tx, err := bdb.db.Begin(false)
	if err != nil {
		panic(err)
	}
	c := tx.Bucket(bucket).Cursor()
	return newBoltDBIterator(c, start, end, false)
}

func (bdb *BoltDB) ReverseIterator(start, end []byte) Iterator {
	tx, err := bdb.db.Begin(false)
	if err != nil {
		panic(err)
	}
	c := tx.Bucket(bucket).Cursor()
	return newBoltDBIterator(c, start, end, true)
}

// boltDBIterator allows you to iterate on range of keys/values given some
// start / end keys (nil & nil will result in doing full scan).
type boltDBIterator struct {
	itr   *bbolt.Cursor
	start []byte
	end   []byte

	//current key
	cKey []byte

	//current value
	cValue []byte

	isInvalid bool
	isReverse bool
}

func newBoltDBIterator(itr *bbolt.Cursor, start, end []byte, isReverse bool) *boltDBIterator {
	var ck, cv []byte
	if isReverse {
		if end == nil {
			ck, cv = itr.Last()
		} else {
			ck, cv = itr.Seek(end)
		}
	} else {
		if start == nil {
			ck, cv = itr.First()
		} else {
			ck, cv = itr.Seek(start)
		}
	}

	return &boltDBIterator{
		itr:       itr,
		start:     start,
		end:       end,
		cKey:      ck,
		cValue:    cv,
		isReverse: isReverse,
		isInvalid: false,
	}
}

func (bdbi *boltDBIterator) Domain() ([]byte, []byte) {
	return bdbi.start, bdbi.end
}

func (bdbi *boltDBIterator) Valid() bool {
	if bdbi.isInvalid {
		return false
	}

	var start = bdbi.start
	var end = bdbi.end
	var key = bdbi.cKey

	if bdbi.isReverse {
		if start != nil && bytes.Compare(key, start) < 0 {
			bdbi.isInvalid = true
			return false
		}
	} else {
		if end != nil && bytes.Compare(end, key) <= 0 {
			bdbi.isInvalid = true
			return false
		}
	}

	// Valid
	return true
}

func (bdbi *boltDBIterator) Next() {
	bdbi.assertIsValid()
	bdbi.cKey, bdbi.cValue = bdbi.itr.Next()
}

func (bdbi *boltDBIterator) Key() []byte {
	bdbi.assertIsValid()
	return bdbi.cKey
}

func (bdbi *boltDBIterator) Value() []byte {
	bdbi.assertIsValid()
	return bdbi.cValue
}

// boltdb cursor has no close op.
func (bdbi *boltDBIterator) Close() {}

func (bdbi *boltDBIterator) assertIsValid() {
	if !bdbi.Valid() {
		panic("Boltdb-iterator is invalid")
	}
}

// nonEmptyKey returns a []byte("nil") if key is empty.
// WARNING: this may collude with "nil" user key!
func nonEmptyKey(key []byte) []byte {
	if len(key) == 0 {
		return []byte("nil")
	}
	return key
}
