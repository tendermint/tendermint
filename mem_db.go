package db

import (
	"fmt"
	"sort"
	"sync"
)

func init() {
	registerDBCreator(MemDBBackend, func(name, dir string) (DB, error) {
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

// Implements atomicSetDeleter.
func (db *MemDB) Mutex() *sync.Mutex {
	return &(db.mtx)
}

// Implements DB.
func (db *MemDB) Get(key []byte) []byte {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	key = nonNilBytes(key)

	value := db.db[string(key)]
	return value
}

// Implements DB.
func (db *MemDB) Has(key []byte) bool {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	key = nonNilBytes(key)

	_, ok := db.db[string(key)]
	return ok
}

// Implements DB.
func (db *MemDB) Set(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
}

// Implements DB.
func (db *MemDB) SetSync(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
}

// Implements atomicSetDeleter.
func (db *MemDB) SetNoLock(key []byte, value []byte) {
	db.SetNoLockSync(key, value)
}

// Implements atomicSetDeleter.
func (db *MemDB) SetNoLockSync(key []byte, value []byte) {
	key = nonNilBytes(key)
	value = nonNilBytes(value)

	db.db[string(key)] = value
}

// Implements DB.
func (db *MemDB) Delete(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
}

// Implements DB.
func (db *MemDB) DeleteSync(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
}

// Implements atomicSetDeleter.
func (db *MemDB) DeleteNoLock(key []byte) {
	db.DeleteNoLockSync(key)
}

// Implements atomicSetDeleter.
func (db *MemDB) DeleteNoLockSync(key []byte) {
	key = nonNilBytes(key)

	delete(db.db, string(key))
}

// Implements DB.
func (db *MemDB) Close() {
	// Close is a noop since for an in-memory
	// database, we don't have a destination
	// to flush contents to nor do we want
	// any data loss on invoking Close()
	// See the discussion in https://github.com/tendermint/tendermint/libs/pull/56
}

// Implements DB.
func (db *MemDB) Print() {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	for key, value := range db.db {
		fmt.Printf("[%X]:\t[%X]\n", []byte(key), value)
	}
}

// Implements DB.
func (db *MemDB) Stats() map[string]string {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	stats := make(map[string]string)
	stats["database.type"] = "memDB"
	stats["database.size"] = fmt.Sprintf("%d", len(db.db))
	return stats
}

// Implements DB.
func (db *MemDB) NewBatch() Batch {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return &memBatch{db, nil}
}

//----------------------------------------
// Iterator

// Implements DB.
func (db *MemDB) Iterator(start, end []byte) Iterator {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	keys := db.getSortedKeys(start, end, false)
	return newMemDBIterator(db, keys, start, end)
}

// Implements DB.
func (db *MemDB) ReverseIterator(start, end []byte) Iterator {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	keys := db.getSortedKeys(start, end, true)
	return newMemDBIterator(db, keys, start, end)
}

// We need a copy of all of the keys.
// Not the best, but probably not a bottleneck depending.
type memDBIterator struct {
	db    DB
	cur   int
	keys  []string
	start []byte
	end   []byte
}

var _ Iterator = (*memDBIterator)(nil)

// Keys is expected to be in reverse order for reverse iterators.
func newMemDBIterator(db DB, keys []string, start, end []byte) *memDBIterator {
	return &memDBIterator{
		db:    db,
		cur:   0,
		keys:  keys,
		start: start,
		end:   end,
	}
}

// Implements Iterator.
func (itr *memDBIterator) Domain() ([]byte, []byte) {
	return itr.start, itr.end
}

// Implements Iterator.
func (itr *memDBIterator) Valid() bool {
	return 0 <= itr.cur && itr.cur < len(itr.keys)
}

// Implements Iterator.
func (itr *memDBIterator) Next() {
	itr.assertIsValid()
	itr.cur++
}

// Implements Iterator.
func (itr *memDBIterator) Key() []byte {
	itr.assertIsValid()
	return []byte(itr.keys[itr.cur])
}

// Implements Iterator.
func (itr *memDBIterator) Value() []byte {
	itr.assertIsValid()
	key := []byte(itr.keys[itr.cur])
	return itr.db.Get(key)
}

// Implements Iterator.
func (itr *memDBIterator) Close() {
	itr.keys = nil
	itr.db = nil
}

func (itr *memDBIterator) assertIsValid() {
	if !itr.Valid() {
		panic("memDBIterator is invalid")
	}
}

//----------------------------------------
// Misc.

func (db *MemDB) getSortedKeys(start, end []byte, reverse bool) []string {
	keys := []string{}
	for key := range db.db {
		inDomain := IsKeyInDomain([]byte(key), start, end)
		if inDomain {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if reverse {
		nkeys := len(keys)
		for i := 0; i < nkeys/2; i++ {
			temp := keys[i]
			keys[i] = keys[nkeys-i-1]
			keys[nkeys-i-1] = temp
		}
	}
	return keys
}
