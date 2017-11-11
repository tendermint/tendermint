// +build gcc

package db

import (
	"fmt"
	"path"

	"github.com/jmhodges/levigo"
)

func init() {
	dbCreator := func(name string, dir string) (DB, error) {
		return NewCLevelDB(name, dir)
	}
	registerDBCreator(LevelDBBackendStr, dbCreator, true)
	registerDBCreator(CLevelDBBackendStr, dbCreator, false)
}

type CLevelDB struct {
	db     *levigo.DB
	ro     *levigo.ReadOptions
	wo     *levigo.WriteOptions
	woSync *levigo.WriteOptions

	cwwMutex
}

func NewCLevelDB(name string, dir string) (*CLevelDB, error) {
	dbPath := path.Join(dir, name+".db")

	opts := levigo.NewOptions()
	opts.SetCache(levigo.NewLRUCache(1 << 30))
	opts.SetCreateIfMissing(true)
	db, err := levigo.Open(dbPath, opts)
	if err != nil {
		return nil, err
	}
	ro := levigo.NewReadOptions()
	wo := levigo.NewWriteOptions()
	woSync := levigo.NewWriteOptions()
	woSync.SetSync(true)
	database := &CLevelDB{
		db:     db,
		ro:     ro,
		wo:     wo,
		woSync: woSync,

		cwwMutex: NewCWWMutex(),
	}
	return database, nil
}

func (db *CLevelDB) Get(key []byte) []byte {
	res, err := db.db.Get(db.ro, key)
	if err != nil {
		panic(err)
	}
	return res
}

func (db *CLevelDB) Set(key []byte, value []byte) {
	err := db.db.Put(db.wo, key, value)
	if err != nil {
		panic(err)
	}
}

func (db *CLevelDB) SetSync(key []byte, value []byte) {
	err := db.db.Put(db.woSync, key, value)
	if err != nil {
		panic(err)
	}
}

func (db *CLevelDB) Delete(key []byte) {
	err := db.db.Delete(db.wo, key)
	if err != nil {
		panic(err)
	}
}

func (db *CLevelDB) DeleteSync(key []byte) {
	err := db.db.Delete(db.woSync, key)
	if err != nil {
		panic(err)
	}
}

func (db *CLevelDB) DB() *levigo.DB {
	return db.db
}

func (db *CLevelDB) Close() {
	db.db.Close()
	db.ro.Close()
	db.wo.Close()
	db.woSync.Close()
}

func (db *CLevelDB) Print() {
	itr := db.Iterator()
	defer itr.Close()
	for itr.Seek(nil); itr.Valid(); itr.Next() {
		key := itr.Key()
		value := itr.Value()
		fmt.Printf("[%X]:\t[%X]\n", key, value)
	}
}

func (db *CLevelDB) Stats() map[string]string {
	// TODO: Find the available properties for the C LevelDB implementation
	keys := []string{}

	stats := make(map[string]string)
	for _, key := range keys {
		str := db.db.PropertyValue(key)
		stats[key] = str
	}
	return stats
}

func (db *CLevelDB) CacheDB() CacheDB {
	return NewCacheDB(db, db.GetWriteLockVersion())
}

//----------------------------------------
// Batch

func (db *CLevelDB) NewBatch() Batch {
	batch := levigo.NewWriteBatch()
	return &cLevelDBBatch{db, batch}
}

type cLevelDBBatch struct {
	db    *CLevelDB
	batch *levigo.WriteBatch
}

func (mBatch *cLevelDBBatch) Set(key, value []byte) {
	mBatch.batch.Put(key, value)
}

func (mBatch *cLevelDBBatch) Delete(key []byte) {
	mBatch.batch.Delete(key)
}

func (mBatch *cLevelDBBatch) Write() {
	err := mBatch.db.db.Write(mBatch.db.wo, mBatch.batch)
	if err != nil {
		panic(err)
	}
}

//----------------------------------------
// Iterator

func (db *CLevelDB) Iterator() Iterator {
	itr := db.db.NewIterator(db.ro)
	itr.Seek([]byte{0x00})
	return cLevelDBIterator{itr}
}

type cLevelDBIterator struct {
	itr *levigo.Iterator
}

func (c cLevelDBIterator) Seek(key []byte) {
	if key == nil {
		key = []byte{0x00}
	}
	c.itr.Seek(key)
}

func (c cLevelDBIterator) Valid() bool {
	return c.itr.Valid()
}

func (c cLevelDBIterator) Key() []byte {
	if !c.itr.Valid() {
		panic("cLevelDBIterator Key() called when invalid")
	}
	return c.itr.Key()
}

func (c cLevelDBIterator) Value() []byte {
	if !c.itr.Valid() {
		panic("cLevelDBIterator Value() called when invalid")
	}
	return c.itr.Value()
}

func (c cLevelDBIterator) Next() {
	if !c.itr.Valid() {
		panic("cLevelDBIterator Next() called when invalid")
	}
	c.itr.Next()
}

func (c cLevelDBIterator) Prev() {
	if !c.itr.Valid() {
		panic("cLevelDBIterator Prev() called when invalid")
	}
	c.itr.Prev()
}

func (c cLevelDBIterator) Close() {
	c.itr.Close()
}

func (c cLevelDBIterator) GetError() error {
	return c.itr.GetError()
}
