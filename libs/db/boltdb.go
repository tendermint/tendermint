// +build boltdb

package db

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/etcd-io/bbolt"
)

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
//
// A single bucket ([]byte("tm")) is used per a database instance. This could
// lead to performance issues when/if there will be lots of keys.
type BoltDB struct {
	db *bbolt.DB
}

// NewBoltDB returns a BoltDB with default options.
func NewBoltDB(name, dir string) (DB, error) {
	return NewBoltDBWithOpts(name, dir, bbolt.DefaultOptions)
}

// NewBoltDBWithOpts allows you to supply *bbolt.Options. ReadOnly: true is not
// supported because NewBoltDBWithOpts creates a global bucket.
func NewBoltDBWithOpts(name string, dir string, opts *bbolt.Options) (DB, error) {
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

func (bdb *BoltDB) Get(key []byte) (value []byte) {
	key = nonEmptyKey(nonNilBytes(key))
	err := bdb.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucket)
		if v := b.Get(key); v != nil {
			value = append([]byte{}, v...)
		}
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

	err := bdb.db.View(func(tx *bbolt.Tx) error {
		tx.Bucket(bucket).ForEach(func(k, v []byte) error {
			fmt.Printf("[%X]:\t[%X]\n", k, v)
			return nil
		})
		return nil
	})
	if err != nil {
		panic(err)
	}
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
	db  *BoltDB
	ops []operation
}

// NewBatch returns a new batch.
func (bdb *BoltDB) NewBatch() Batch {
	return &boltDBBatch{
		ops: nil,
		db:  bdb,
	}
}

// It is safe to modify the contents of the argument after Set returns but not
// before.
func (bdb *boltDBBatch) Set(key, value []byte) {
	bdb.ops = append(bdb.ops, operation{opTypeSet, key, value})
}

// It is safe to modify the contents of the argument after Delete returns but
// not before.
func (bdb *boltDBBatch) Delete(key []byte) {
	bdb.ops = append(bdb.ops, operation{opTypeDelete, key, nil})
}

// NOTE: the operation is synchronous (see BoltDB for reasons)
func (bdb *boltDBBatch) Write() {
	err := bdb.db.db.Batch(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucket)
		for _, op := range bdb.ops {
			key := nonEmptyKey(nonNilBytes(op.key))
			switch op.opType {
			case opTypeSet:
				if putErr := b.Put(key, op.value); putErr != nil {
					return putErr
				}
			case opTypeDelete:
				if delErr := b.Delete(key); delErr != nil {
					return delErr
				}
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
}

func (bdb *boltDBBatch) WriteSync() {
	bdb.Write()
}

func (bdb *boltDBBatch) Close() {}

// WARNING: Any concurrent writes or reads will block until the iterator is
// closed.
func (bdb *BoltDB) Iterator(start, end []byte) Iterator {
	tx, err := bdb.db.Begin(false)
	if err != nil {
		panic(err)
	}
	return newBoltDBIterator(tx, start, end, false)
}

// WARNING: Any concurrent writes or reads will block until the iterator is
// closed.
func (bdb *BoltDB) ReverseIterator(start, end []byte) Iterator {
	tx, err := bdb.db.Begin(false)
	if err != nil {
		panic(err)
	}
	return newBoltDBIterator(tx, start, end, true)
}

// boltDBIterator allows you to iterate on range of keys/values given some
// start / end keys (nil & nil will result in doing full scan).
type boltDBIterator struct {
	tx *bbolt.Tx

	itr   *bbolt.Cursor
	start []byte
	end   []byte

	currentKey   []byte
	currentValue []byte

	isInvalid bool
	isReverse bool
}

func newBoltDBIterator(tx *bbolt.Tx, start, end []byte, isReverse bool) *boltDBIterator {
	itr := tx.Bucket(bucket).Cursor()

	var ck, cv []byte
	if isReverse {
		if end == nil {
			ck, cv = itr.Last()
		} else {
			_, _ = itr.Seek(end) // after key
			ck, cv = itr.Prev()  // return to end key
		}
	} else {
		if start == nil {
			ck, cv = itr.First()
		} else {
			ck, cv = itr.Seek(start)
		}
	}

	return &boltDBIterator{
		tx:           tx,
		itr:          itr,
		start:        start,
		end:          end,
		currentKey:   ck,
		currentValue: cv,
		isReverse:    isReverse,
		isInvalid:    false,
	}
}

func (itr *boltDBIterator) Domain() ([]byte, []byte) {
	return itr.start, itr.end
}

func (itr *boltDBIterator) Valid() bool {
	if itr.isInvalid {
		return false
	}

	// iterated to the end of the cursor
	if len(itr.currentKey) == 0 {
		itr.isInvalid = true
		return false
	}

	if itr.isReverse {
		if itr.start != nil && bytes.Compare(itr.currentKey, itr.start) < 0 {
			itr.isInvalid = true
			return false
		}
	} else {
		if itr.end != nil && bytes.Compare(itr.end, itr.currentKey) <= 0 {
			itr.isInvalid = true
			return false
		}
	}

	// Valid
	return true
}

func (itr *boltDBIterator) Next() {
	itr.assertIsValid()
	if itr.isReverse {
		itr.currentKey, itr.currentValue = itr.itr.Prev()
	} else {
		itr.currentKey, itr.currentValue = itr.itr.Next()
	}
}

func (itr *boltDBIterator) Key() []byte {
	itr.assertIsValid()
	return append([]byte{}, itr.currentKey...)
}

func (itr *boltDBIterator) Value() []byte {
	itr.assertIsValid()
	var value []byte
	if itr.currentValue != nil {
		value = append([]byte{}, itr.currentValue...)
	}
	return value
}

func (itr *boltDBIterator) Close() {
	err := itr.tx.Rollback()
	if err != nil {
		panic(err)
	}
}

func (itr *boltDBIterator) assertIsValid() {
	if !itr.Valid() {
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
