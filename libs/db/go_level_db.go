package db

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"

	cmn "github.com/tendermint/tendermint/libs/common"
)

func init() {
	dbCreator := func(name string, dir string) (DB, error) {
		return NewGoLevelDB(name, dir)
	}
	registerDBCreator(LevelDBBackend, dbCreator, false)
	registerDBCreator(GoLevelDBBackend, dbCreator, false)
}

var _ DB = (*GoLevelDB)(nil)

type GoLevelDB struct {
	db *leveldb.DB
}

func NewGoLevelDB(name string, dir string) (*GoLevelDB, error) {
	return NewGoLevelDBWithOpts(name, dir, nil)
}

func NewGoLevelDBWithOpts(name string, dir string, o *opt.Options) (*GoLevelDB, error) {
	dbPath := filepath.Join(dir, name+".db")
	db, err := leveldb.OpenFile(dbPath, o)
	if err != nil {
		return nil, err
	}
	database := &GoLevelDB{
		db: db,
	}
	return database, nil
}

// Implements DB.
func (db *GoLevelDB) Get(key []byte) []byte {
	key = nonNilBytes(key)
	res, err := db.db.Get(key, nil)
	if err != nil {
		if err == errors.ErrNotFound {
			return nil
		}
		panic(err)
	}
	return res
}

// Implements DB.
func (db *GoLevelDB) Has(key []byte) bool {
	return db.Get(key) != nil
}

// Implements DB.
func (db *GoLevelDB) Set(key []byte, value []byte) {
	key = nonNilBytes(key)
	value = nonNilBytes(value)
	err := db.db.Put(key, value, nil)
	if err != nil {
		cmn.PanicCrisis(err)
	}
}

// Implements DB.
func (db *GoLevelDB) SetSync(key []byte, value []byte) {
	key = nonNilBytes(key)
	value = nonNilBytes(value)
	err := db.db.Put(key, value, &opt.WriteOptions{Sync: true})
	if err != nil {
		cmn.PanicCrisis(err)
	}
}

// Implements DB.
func (db *GoLevelDB) Delete(key []byte) {
	key = nonNilBytes(key)
	err := db.db.Delete(key, nil)
	if err != nil {
		cmn.PanicCrisis(err)
	}
}

// Implements DB.
func (db *GoLevelDB) DeleteSync(key []byte) {
	key = nonNilBytes(key)
	err := db.db.Delete(key, &opt.WriteOptions{Sync: true})
	if err != nil {
		cmn.PanicCrisis(err)
	}
}

func (db *GoLevelDB) DB() *leveldb.DB {
	return db.db
}

// Implements DB.
func (db *GoLevelDB) Close() {
	db.db.Close()
}

// Implements DB.
func (db *GoLevelDB) Print() {
	str, _ := db.db.GetProperty("leveldb.stats")
	fmt.Printf("%v\n", str)

	itr := db.db.NewIterator(nil, nil)
	for itr.Next() {
		key := itr.Key()
		value := itr.Value()
		fmt.Printf("[%X]:\t[%X]\n", key, value)
	}
}

// Implements DB.
func (db *GoLevelDB) Stats() map[string]string {
	keys := []string{
		"leveldb.num-files-at-level{n}",
		"leveldb.stats",
		"leveldb.sstables",
		"leveldb.blockpool",
		"leveldb.cachedblock",
		"leveldb.openedtables",
		"leveldb.alivesnaps",
		"leveldb.aliveiters",
	}

	stats := make(map[string]string)
	for _, key := range keys {
		str, err := db.db.GetProperty(key)
		if err == nil {
			stats[key] = str
		}
	}
	return stats
}

//----------------------------------------
// Batch

// Implements DB.
func (db *GoLevelDB) NewBatch() Batch {
	batch := new(leveldb.Batch)
	return &goLevelDBBatch{db, batch}
}

type goLevelDBBatch struct {
	db    *GoLevelDB
	batch *leveldb.Batch
}

// Implements Batch.
func (mBatch *goLevelDBBatch) Set(key, value []byte) {
	mBatch.batch.Put(key, value)
}

// Implements Batch.
func (mBatch *goLevelDBBatch) Delete(key []byte) {
	mBatch.batch.Delete(key)
}

// Implements Batch.
func (mBatch *goLevelDBBatch) Write() {
	err := mBatch.db.db.Write(mBatch.batch, &opt.WriteOptions{Sync: false})
	if err != nil {
		panic(err)
	}
}

// Implements Batch.
func (mBatch *goLevelDBBatch) WriteSync() {
	err := mBatch.db.db.Write(mBatch.batch, &opt.WriteOptions{Sync: true})
	if err != nil {
		panic(err)
	}
}

//----------------------------------------
// Iterator
// NOTE This is almost identical to db/c_level_db.Iterator
// Before creating a third version, refactor.

// Implements DB.
func (db *GoLevelDB) Iterator(start, end []byte) Iterator {
	itr := db.db.NewIterator(nil, nil)
	return newGoLevelDBIterator(itr, start, end, false)
}

// Implements DB.
func (db *GoLevelDB) ReverseIterator(start, end []byte) Iterator {
	itr := db.db.NewIterator(nil, nil)
	return newGoLevelDBIterator(itr, start, end, true)
}

type goLevelDBIterator struct {
	source    iterator.Iterator
	start     []byte
	end       []byte
	isReverse bool
	isInvalid bool
}

var _ Iterator = (*goLevelDBIterator)(nil)

func newGoLevelDBIterator(source iterator.Iterator, start, end []byte, isReverse bool) *goLevelDBIterator {
	if isReverse {
		if start == nil {
			source.Last()
		} else {
			valid := source.Seek(start)
			if valid {
				soakey := source.Key() // start or after key
				if bytes.Compare(start, soakey) < 0 {
					source.Prev()
				}
			} else {
				source.Last()
			}
		}
	} else {
		if start == nil {
			source.First()
		} else {
			source.Seek(start)
		}
	}
	return &goLevelDBIterator{
		source:    source,
		start:     start,
		end:       end,
		isReverse: isReverse,
		isInvalid: false,
	}
}

// Implements Iterator.
func (itr *goLevelDBIterator) Domain() ([]byte, []byte) {
	return itr.start, itr.end
}

// Implements Iterator.
func (itr *goLevelDBIterator) Valid() bool {

	// Once invalid, forever invalid.
	if itr.isInvalid {
		return false
	}

	// Panic on DB error.  No way to recover.
	itr.assertNoError()

	// If source is invalid, invalid.
	if !itr.source.Valid() {
		itr.isInvalid = true
		return false
	}

	// If key is end or past it, invalid.
	var end = itr.end
	var key = itr.source.Key()

	if itr.isReverse {
		if end != nil && bytes.Compare(key, end) <= 0 {
			itr.isInvalid = true
			return false
		}
	} else {
		if end != nil && bytes.Compare(end, key) <= 0 {
			itr.isInvalid = true
			return false
		}
	}

	// Valid
	return true
}

// Implements Iterator.
func (itr *goLevelDBIterator) Key() []byte {
	// Key returns a copy of the current key.
	// See https://github.com/syndtr/goleveldb/blob/52c212e6c196a1404ea59592d3f1c227c9f034b2/leveldb/iterator/iter.go#L88
	itr.assertNoError()
	itr.assertIsValid()
	return cp(itr.source.Key())
}

// Implements Iterator.
func (itr *goLevelDBIterator) Value() []byte {
	// Value returns a copy of the current value.
	// See https://github.com/syndtr/goleveldb/blob/52c212e6c196a1404ea59592d3f1c227c9f034b2/leveldb/iterator/iter.go#L88
	itr.assertNoError()
	itr.assertIsValid()
	return cp(itr.source.Value())
}

// Implements Iterator.
func (itr *goLevelDBIterator) Next() {
	itr.assertNoError()
	itr.assertIsValid()
	if itr.isReverse {
		itr.source.Prev()
	} else {
		itr.source.Next()
	}
}

// Implements Iterator.
func (itr *goLevelDBIterator) Close() {
	itr.source.Release()
}

func (itr *goLevelDBIterator) assertNoError() {
	if err := itr.source.Error(); err != nil {
		panic(err)
	}
}

func (itr goLevelDBIterator) assertIsValid() {
	if !itr.Valid() {
		panic("goLevelDBIterator is invalid")
	}
}
