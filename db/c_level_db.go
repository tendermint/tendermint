// +build gcc

package db

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/jmhodges/levigo"
)

func init() {
	dbCreator := func(name string, dir string) (DB, error) {
		return NewCLevelDB(name, dir)
	}
	registerDBCreator(LevelDBBackendStr, dbCreator, true)
	registerDBCreator(CLevelDBBackendStr, dbCreator, false)
}

var _ DB = (*CLevelDB)(nil)

type CLevelDB struct {
	db     *levigo.DB
	ro     *levigo.ReadOptions
	wo     *levigo.WriteOptions
	woSync *levigo.WriteOptions
}

func NewCLevelDB(name string, dir string) (*CLevelDB, error) {
	dbPath := filepath.Join(dir, name+".db")

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
	}
	return database, nil
}

func (db *CLevelDB) Get(key []byte) []byte {
	panicNilKey(key)
	res, err := db.db.Get(db.ro, key)
	if err != nil {
		panic(err)
	}
	return res
}

func (db *CLevelDB) Has(key []byte) bool {
	panicNilKey(key)
	panic("not implemented yet")
}

func (db *CLevelDB) Set(key []byte, value []byte) {
	panicNilKey(key)
	err := db.db.Put(db.wo, key, value)
	if err != nil {
		panic(err)
	}
}

func (db *CLevelDB) SetSync(key []byte, value []byte) {
	panicNilKey(key)
	err := db.db.Put(db.woSync, key, value)
	if err != nil {
		panic(err)
	}
}

func (db *CLevelDB) Delete(key []byte) {
	panicNilKey(key)
	err := db.db.Delete(db.wo, key)
	if err != nil {
		panic(err)
	}
}

func (db *CLevelDB) DeleteSync(key []byte) {
	panicNilKey(key)
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
	itr := db.Iterator(BeginningKey(), EndingKey())
	defer itr.Release()
	for ; itr.Valid(); itr.Next() {
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

func (db *CLevelDB) Iterator(start, end []byte) Iterator {
	itr := db.db.NewIterator(db.ro)
	if len(start) > 0 {
		itr.Seek(start)
	} else {
		itr.SeekToFirst()
	}
	return &cLevelDBIterator{
		itr:   itr,
		start: start,
		end:   end,
	}
}

func (db *CLevelDB) ReverseIterator(start, end []byte) Iterator {
	// XXX
	return nil
}

var _ Iterator = (*cLevelDBIterator)(nil)

type cLevelDBIterator struct {
	itr        *levigo.Iterator
	start, end []byte
	invalid    bool
}

func (c *cLevelDBIterator) Domain() ([]byte, []byte) {
	return c.start, c.end
}

func (c *cLevelDBIterator) Valid() bool {
	c.assertNoError()
	if c.invalid {
		return false
	}
	c.invalid = !c.itr.Valid()
	return !c.invalid
}

func (c *cLevelDBIterator) Key() []byte {
	if !c.Valid() {
		panic("cLevelDBIterator Key() called when invalid")
	}
	return c.itr.Key()
}

func (c *cLevelDBIterator) Value() []byte {
	if !c.Valid() {
		panic("cLevelDBIterator Value() called when invalid")
	}
	return c.itr.Value()
}

func (c *cLevelDBIterator) Next() {
	if !c.Valid() {
		panic("cLevelDBIterator Next() called when invalid")
	}
	c.itr.Next()
	c.checkEndKey() // if we've exceeded the range, we're now invalid
}

// levigo has no upper bound when iterating, so need to check ourselves
func (c *cLevelDBIterator) checkEndKey() []byte {
	key := c.itr.Key()
	if c.end != nil && bytes.Compare(key, c.end) > 0 {
		c.invalid = true
	}
	return key
}

func (c *cLevelDBIterator) Release() {
	c.itr.Close()
}

func (c *cLevelDBIterator) assertNoError() {
	if err := c.itr.GetError(); err != nil {
		panic(err)
	}
}
