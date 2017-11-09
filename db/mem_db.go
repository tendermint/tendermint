package db

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
)

func init() {
	registerDBCreator(MemDBBackendStr, func(name string, dir string) (DB, error) {
		return NewMemDB(), nil
	}, false)
}

type MemDB struct {
	mtx sync.Mutex
	db  map[string][]byte

	cwwMutex
}

func NewMemDB() *MemDB {
	database := &MemDB{
		db:       make(map[string][]byte),
		cwwMutex: NewCWWMutex(),
	}
	return database
}

func (db *MemDB) Get(key []byte) []byte {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.db[string(key)]
}

func (db *MemDB) Set(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
}

func (db *MemDB) SetSync(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
}

// NOTE: Implements atomicSetDeleter
func (db *MemDB) SetNoLock(key []byte, value []byte) {
	if value == nil {
		value = []byte{}
	}
	db.db[string(key)] = value
}

func (db *MemDB) Delete(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	delete(db.db, string(key))
}

func (db *MemDB) DeleteSync(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	delete(db.db, string(key))
}

// NOTE: Implements atomicSetDeleter
func (db *MemDB) DeleteNoLock(key []byte) {
	delete(db.db, string(key))
}

func (db *MemDB) Close() {
	// Close is a noop since for an in-memory
	// database, we don't have a destination
	// to flush contents to nor do we want
	// any data loss on invoking Close()
	// See the discussion in https://github.com/tendermint/tmlibs/pull/56
}

func (db *MemDB) Print() {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	for key, value := range db.db {
		fmt.Printf("[%X]:\t[%X]\n", []byte(key), value)
	}
}

func (db *MemDB) Stats() map[string]string {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	stats := make(map[string]string)
	stats["database.type"] = "memDB"
	stats["database.size"] = fmt.Sprintf("%d", len(db.db))
	return stats
}

func (db *MemDB) NewBatch() Batch {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return &memBatch{db, nil}
}

func (db *MemDB) Mutex() *sync.Mutex {
	return &(db.mtx)
}

func (db *MemDB) CacheWrap() interface{} {
	return NewCacheDB(db, db.GetWriteLockVersion())
}

//----------------------------------------

func (db *MemDB) Iterator() Iterator {
	it := newMemDBIterator()
	it.db = db
	it.cur = 0

	db.mtx.Lock()
	defer db.mtx.Unlock()

	// We need a copy of all of the keys.
	// Not the best, but probably not a bottleneck depending.
	for key, _ := range db.db {
		it.keys = append(it.keys, key)
	}
	sort.Strings(it.keys)
	return it
}

type memDBIterator struct {
	cur  int
	keys []string
	db   DB
}

func newMemDBIterator() *memDBIterator {
	return &memDBIterator{}
}

func (it *memDBIterator) Seek(key []byte) {
	for i, ik := range it.keys {
		it.cur = i
		if bytes.Compare(key, []byte(ik)) <= 0 {
			return
		}
	}
	it.cur += 1 // If not found, becomes invalid.
}

func (it *memDBIterator) Valid() bool {
	return 0 <= it.cur && it.cur < len(it.keys)
}

func (it *memDBIterator) Next() {
	if !it.Valid() {
		panic("memDBIterator Next() called when invalid")
	}
	it.cur++
}

func (it *memDBIterator) Prev() {
	if !it.Valid() {
		panic("memDBIterator Next() called when invalid")
	}
	it.cur--
}

func (it *memDBIterator) Key() []byte {
	if !it.Valid() {
		panic("memDBIterator Key() called when invalid")
	}
	return []byte(it.keys[it.cur])
}

func (it *memDBIterator) Value() []byte {
	if !it.Valid() {
		panic("memDBIterator Value() called when invalid")
	}
	return it.db.Get(it.Key())
}

func (it *memDBIterator) Close() {
	it.db = nil
	it.keys = nil
}

func (it *memDBIterator) GetError() error {
	return nil
}
