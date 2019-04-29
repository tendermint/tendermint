package db

import (
	"bytes"
	"github.com/etcd-io/bbolt"
	"path/filepath"
	"sync"
)

var bucket = []byte("boltdb_bucket")

func init() {
	registerDBCreator(BoltDBBackend, func(name string, dir string) (DB, error) {
		return NewBoltdb(name, dir)
	}, false)
}

type Boltdb struct {
	db *bbolt.DB
}

func NewBoltdb(name, dir string) (DB, error) {
	return NewBoltdbWithOpts(name, dir, bbolt.DefaultOptions)
}

func NewBoltdbWithOpts(name, dir string, opts *bbolt.Options) (DB, error) {
	dbPath := filepath.Join(dir, name+".db")
	db, err := bbolt.Open(dbPath, 0600, opts)
	if err != nil {
		return nil, err
	}
	return &Boltdb{db: db}, nil
}

func (bdb *Boltdb) Get(key []byte) (value []byte) {
	key = nonNilBytes(key)
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

func (bdb *Boltdb) Has(key []byte) bool {
	return bdb.Get(key) != nil
}

func (bdb *Boltdb) Set(key, value []byte) {
	key = nonNilBytes(key)
	value = nonNilBytes(value)
	err := bdb.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucket)
		return b.Put(key, value)
	})
	if err != nil {
		panic(err)
	}
}

func (bdb *Boltdb) SetSync(key, value []byte) {
	bdb.Set(key, value)
}

func (bdb *Boltdb) Delete(key []byte) {
	key = nonNilBytes(key)
	err := bdb.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucket).Delete(key)
	})
	if err != nil {
		panic(err)
	}
}

func (bdb *Boltdb) DeleteSync(key []byte) {
	bdb.Delete(key)
}

func (bdb *Boltdb) Close() {
	bdb.db.Close()
}

func (bdb *Boltdb) Print() {
	panic("boltdb.print not yet implemented")
}

func (bdb *Boltdb) Stats() map[string]string {
	panic("boltdb.stats not yet implemented")
}

type BoltdbBatch struct {
	buffer *sync.Map
	db     *Boltdb
}

func (bdb *Boltdb) NewBatch() Batch {
	return &BoltdbBatch{
		buffer: &sync.Map{},
		db:     bdb,
	}
}

func (bdb *BoltdbBatch) Set(key, value []byte) {
	bdb.buffer.Store(key, value)
}

func (bdb *BoltdbBatch) Delete(key []byte) {
	bdb.buffer.Delete(key)
}

func (bdb *BoltdbBatch) Write() {
	err := bdb.db.db.Batch(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucket)
		bdb.buffer.Range(func(key, value interface{}) bool {
			b.Put(key.([]byte), value.([]byte))
			return true
		})
		return nil
	})
	if err != nil {
		panic(err)
	}
}

func (bdb *BoltdbBatch) WriteSync() {
	bdb.Write()
}

func (bdb *BoltdbBatch) Close() {}

func (bdb *Boltdb) Iterator(start, end []byte) Iterator {
	tx, err := bdb.db.Begin(false)
	if err != nil {
		panic(err)
	}
	c := tx.Bucket(bucket).Cursor()
	return newBoltdbIterator(c, start, end, false)
}

func (bdb *Boltdb) ReverseIterator(start, end []byte) Iterator {
	tx, err := bdb.db.Begin(false)
	if err != nil {
		panic(err)
	}
	c := tx.Bucket(bucket).Cursor()
	return newBoltdbIterator(c, start, end, true)
}

type BoltdbIterator struct {
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

func newBoltdbIterator(itr *bbolt.Cursor, start, end []byte, isReverse bool) *BoltdbIterator {
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

	return &BoltdbIterator{
		itr:       itr,
		start:     start,
		end:       end,
		cKey:      ck,
		cValue:    cv,
		isReverse: isReverse,
		isInvalid: false,
	}
}

func (bdbi *BoltdbIterator) Domain() ([]byte, []byte) {
	return bdbi.start, bdbi.end
}

func (bdbi *BoltdbIterator) Valid() bool {
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

func (bdbi *BoltdbIterator) Next() {
	bdbi.assertIsValid()
	bdbi.cKey, bdbi.cValue = bdbi.itr.Next()
}

func (bdbi *BoltdbIterator) Key() []byte {
	bdbi.assertIsValid()
	return bdbi.cKey
}

func (bdbi *BoltdbIterator) Value() []byte {
	bdbi.assertIsValid()
	return bdbi.cValue
}

// boltdb cursor has no close op.
func (bdbi *BoltdbIterator) Close() {}

func (bdbi *BoltdbIterator) assertIsValid() {
	if !bdbi.Valid() {
		panic("Boltdb-iterator is invalid")
	}
}
