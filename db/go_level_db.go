package db

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"

	. "github.com/tendermint/tmlibs/common"
)

func init() {
	dbCreator := func(name string, dir string) (DB, error) {
		return NewGoLevelDB(name, dir)
	}
	registerDBCreator(LevelDBBackendStr, dbCreator, false)
	registerDBCreator(GoLevelDBBackendStr, dbCreator, false)
}

var _ DB = (*GoLevelDB)(nil)

type GoLevelDB struct {
	db *leveldb.DB
}

func NewGoLevelDB(name string, dir string) (*GoLevelDB, error) {
	dbPath := filepath.Join(dir, name+".db")
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		return nil, err
	}
	database := &GoLevelDB{
		db: db,
	}
	return database, nil
}

func (db *GoLevelDB) Get(key []byte) []byte {
	key = nonNilBytes(key)
	res, err := db.db.Get(key, nil)
	if err != nil {
		if err == errors.ErrNotFound {
			return nil
		} else {
			panic(err)
		}
	}
	return res
}

func (db *GoLevelDB) Has(key []byte) bool {
	return db.Get(key) != nil
}

func (db *GoLevelDB) Set(key []byte, value []byte) {
	key = nonNilBytes(key)
	value = nonNilBytes(value)
	err := db.db.Put(key, value, nil)
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *GoLevelDB) SetSync(key []byte, value []byte) {
	key = nonNilBytes(key)
	value = nonNilBytes(value)
	err := db.db.Put(key, value, &opt.WriteOptions{Sync: true})
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *GoLevelDB) Delete(key []byte) {
	key = nonNilBytes(key)
	err := db.db.Delete(key, nil)
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *GoLevelDB) DeleteSync(key []byte) {
	key = nonNilBytes(key)
	err := db.db.Delete(key, &opt.WriteOptions{Sync: true})
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *GoLevelDB) DB() *leveldb.DB {
	return db.db
}

func (db *GoLevelDB) Close() {
	db.db.Close()
}

func (db *GoLevelDB) Print() {
	str, _ := db.db.GetProperty("leveldb.stats")
	fmt.Printf("%v\n", str)

	iter := db.db.NewIterator(nil, nil)
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		fmt.Printf("[%X]:\t[%X]\n", key, value)
	}
}

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

func (db *GoLevelDB) NewBatch() Batch {
	batch := new(leveldb.Batch)
	return &goLevelDBBatch{db, batch}
}

type goLevelDBBatch struct {
	db    *GoLevelDB
	batch *leveldb.Batch
}

func (mBatch *goLevelDBBatch) Set(key, value []byte) {
	mBatch.batch.Put(key, value)
}

func (mBatch *goLevelDBBatch) Delete(key []byte) {
	mBatch.batch.Delete(key)
}

func (mBatch *goLevelDBBatch) Write() {
	err := mBatch.db.db.Write(mBatch.batch, nil)
	if err != nil {
		PanicCrisis(err)
	}
}

//----------------------------------------
// Iterator
// NOTE This is almost identical to db/c_level_db.Iterator
// Before creating a third version, refactor.

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
		panic("not implemented yet") // XXX
	}
	source.Seek(start)
	return &goLevelDBIterator{
		source:    source,
		start:     start,
		end:       end,
		isReverse: isReverse,
		isInvalid: false,
	}
}

func (db *GoLevelDB) Iterator(start, end []byte) Iterator {
	itr := db.db.NewIterator(nil, nil)
	return newGoLevelDBIterator(itr, start, end, false)
}

func (db *GoLevelDB) ReverseIterator(start, end []byte) Iterator {
	panic("not implemented yet") // XXX
}

func (itr *goLevelDBIterator) Domain() ([]byte, []byte) {
	return itr.start, itr.end
}

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
	if end != nil && bytes.Compare(end, key) <= 0 {
		itr.isInvalid = true
		return false
	}

	// Valid
	return true
}

func (itr *goLevelDBIterator) Key() []byte {
	itr.assertNoError()
	itr.assertIsValid()
	return itr.source.Key()
}

func (itr *goLevelDBIterator) Value() []byte {
	itr.assertNoError()
	itr.assertIsValid()
	return itr.source.Value()
}

func (itr *goLevelDBIterator) Next() {
	itr.assertNoError()
	itr.assertIsValid()
	itr.source.Next()
}

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
