package db

import (
	"fmt"
	"path"

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
	dbPath := path.Join(dir, name+".db")
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
	panicNilKey(key)
	res, err := db.db.Get(key, nil)
	if err != nil {
		if err == errors.ErrNotFound {
			return nil
		} else {
			PanicCrisis(err)
		}
	}
	return res
}

func (db *GoLevelDB) Has(key []byte) bool {
	panicNilKey(key)
	_, err := db.db.Get(key, nil)
	if err != nil {
		if err == errors.ErrNotFound {
			return false
		} else {
			PanicCrisis(err)
		}
	}
	return true
}

func (db *GoLevelDB) Set(key []byte, value []byte) {
	panicNilKey(key)
	err := db.db.Put(key, value, nil)
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *GoLevelDB) SetSync(key []byte, value []byte) {
	panicNilKey(key)
	err := db.db.Put(key, value, &opt.WriteOptions{Sync: true})
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *GoLevelDB) Delete(key []byte) {
	panicNilKey(key)
	err := db.db.Delete(key, nil)
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *GoLevelDB) DeleteSync(key []byte) {
	panicNilKey(key)
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

func (db *GoLevelDB) Iterator(start, end []byte) Iterator {
	itr := &goLevelDBIterator{
		source: db.db.NewIterator(nil, nil),
		start:  start,
		end:    end,
	}
	itr.source.Seek(start)
	return itr
}

func (db *GoLevelDB) ReverseIterator(start, end []byte) Iterator {
	// XXX
	return nil
}

var _ Iterator = (*goLevelDBIterator)(nil)

type goLevelDBIterator struct {
	source     iterator.Iterator
	invalid    bool
	start, end []byte
}

func (it *goLevelDBIterator) Domain() ([]byte, []byte) {
	return it.start, it.end
}

// Key returns a copy of the current key.
func (it *goLevelDBIterator) Key() []byte {
	if !it.Valid() {
		panic("goLevelDBIterator Key() called when invalid")
	}
	key := it.source.Key()
	k := make([]byte, len(key))
	copy(k, key)

	return k
}

// Value returns a copy of the current value.
func (it *goLevelDBIterator) Value() []byte {
	if !it.Valid() {
		panic("goLevelDBIterator Value() called when invalid")
	}
	val := it.source.Value()
	v := make([]byte, len(val))
	copy(v, val)

	return v
}

func (it *goLevelDBIterator) Valid() bool {
	it.assertNoError()
	if it.invalid {
		return false
	}
	it.invalid = !it.source.Valid()
	return !it.invalid
}

func (it *goLevelDBIterator) Next() {
	if !it.Valid() {
		panic("goLevelDBIterator Next() called when invalid")
	}
	it.source.Next()
}

func (it *goLevelDBIterator) Prev() {
	if !it.Valid() {
		panic("goLevelDBIterator Prev() called when invalid")
	}
	it.source.Prev()
}

func (it *goLevelDBIterator) Close() {
	it.source.Release()
}

func (it *goLevelDBIterator) Release() {
	it.source.Release()
}

func (it *goLevelDBIterator) assertNoError() {
	if err := it.source.Error(); err != nil {
		panic(err)
	}
}
