package db

import (
	"fmt"
	"sort"
	"sync"
)

func init() {
	registerDBCreator(MemDBBackendStr, func(name string, dir string) (DB, error) {
		return NewMemDB(), nil
	}, false)
}

var _ DB = (*MemDB)(nil)

type MemDB struct {
	mtx sync.Mutex
	db  map[string][]byte
}

func NewMemDB() *MemDB {
	database := &MemDB{
		db: make(map[string][]byte),
	}
	return database
}

func (db *MemDB) Get(key []byte) []byte {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	panicNilKey(key)
	return db.db[string(key)]
}

func (db *MemDB) Has(key []byte) bool {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	panicNilKey(key)
	_, ok := db.db[string(key)]
	return ok
}

func (db *MemDB) Set(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	panicNilKey(key)
	db.SetNoLock(key, value)
}

func (db *MemDB) SetSync(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	panicNilKey(key)
	db.SetNoLock(key, value)
}

// NOTE: Implements atomicSetDeleter
func (db *MemDB) SetNoLock(key []byte, value []byte) {
	if value == nil {
		value = []byte{}
	}
	panicNilKey(key)
	db.db[string(key)] = value
}

func (db *MemDB) Delete(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	panicNilKey(key)
	delete(db.db, string(key))
}

func (db *MemDB) DeleteSync(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	panicNilKey(key)
	delete(db.db, string(key))
}

// NOTE: Implements atomicSetDeleter
func (db *MemDB) DeleteNoLock(key []byte) {
	panicNilKey(key)
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

//----------------------------------------

func (db *MemDB) Iterator(start, end []byte) Iterator {
	it := newMemDBIterator(db, start, end)

	db.mtx.Lock()
	defer db.mtx.Unlock()

	// We need a copy of all of the keys.
	// Not the best, but probably not a bottleneck depending.
	it.keys = db.getSortedKeys(start, end)
	return it
}

func (db *MemDB) ReverseIterator(start, end []byte) Iterator {
	it := newMemDBIterator(db, start, end)

	db.mtx.Lock()
	defer db.mtx.Unlock()

	// We need a copy of all of the keys.
	// Not the best, but probably not a bottleneck depending.
	it.keys = db.getSortedKeys(end, start)
	// reverse the order
	l := len(it.keys) - 1
	for i, v := range it.keys {
		it.keys[i] = it.keys[l-i]
		it.keys[l-i] = v
	}
	return nil
}

func (db *MemDB) getSortedKeys(start, end []byte) []string {
	keys := []string{}
	for key, _ := range db.db {
		if IsKeyInDomain([]byte(key), start, end) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

var _ Iterator = (*memDBIterator)(nil)

type memDBIterator struct {
	cur        int
	keys       []string
	db         DB
	start, end []byte
}

func newMemDBIterator(db DB, start, end []byte) *memDBIterator {
	return &memDBIterator{
		db:    db,
		start: start,
		end:   end,
	}
}

func (it *memDBIterator) Domain() ([]byte, []byte) {
	return it.start, it.end
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
