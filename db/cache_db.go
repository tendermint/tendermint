package db

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

// If value is nil but deleted is false,
// it means the parent doesn't have the key.
// (No need to delete upon Write())
type cDBValue struct {
	value   []byte
	deleted bool
	dirty   bool
}

// cacheDB wraps an in-memory cache around an underlying DB.
type cacheDB struct {
	mtx         sync.Mutex
	cache       map[string]cDBValue
	parent      DB
	lockVersion interface{}

	cwwMutex
}

// Needed by MultiStore.CacheWrap().
var _ atomicSetDeleter = (*cacheDB)(nil)
var _ CacheDB = (*cacheDB)(nil)

// Users should typically not be required to call NewCacheDB directly, as the
// DB implementations here provide a .CacheDB() function already.
// `lockVersion` is typically provided by parent.GetWriteLockVersion().
func NewCacheDB(parent DB, lockVersion interface{}) CacheDB {
	db := &cacheDB{
		cache:       make(map[string]cDBValue),
		parent:      parent,
		lockVersion: lockVersion,
		cwwMutex:    NewCWWMutex(),
	}
	return db
}

func (db *cacheDB) Get(key []byte) []byte {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	dbValue, ok := db.cache[string(key)]
	if !ok {
		data := db.parent.Get(key)
		dbValue = cDBValue{value: data, deleted: false, dirty: false}
		db.cache[string(key)] = dbValue
	}
	return dbValue.value
}

func (db *cacheDB) Set(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
}

func (db *cacheDB) SetSync(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
}

func (db *cacheDB) SetNoLock(key []byte, value []byte) {
	db.cache[string(key)] = cDBValue{value: value, deleted: false, dirty: true}
}

func (db *cacheDB) Delete(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
}

func (db *cacheDB) DeleteSync(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
}

func (db *cacheDB) DeleteNoLock(key []byte) {
	db.cache[string(key)] = cDBValue{value: nil, deleted: true, dirty: true}
}

func (db *cacheDB) Close() {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.parent.Close()
}

func (db *cacheDB) Print() {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	fmt.Println("cacheDB\ncache:")
	for key, value := range db.cache {
		fmt.Printf("[%X]:\t[%v]\n", []byte(key), value)
	}
	fmt.Println("\nparent:")
	db.parent.Print()
}

func (db *cacheDB) Stats() map[string]string {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	stats := make(map[string]string)
	stats["cache.size"] = fmt.Sprintf("%d", len(db.cache))
	stats["cache.lock_version"] = fmt.Sprintf("%v", db.lockVersion)
	mergeStats(db.parent.Stats(), stats, "parent.")
	return stats
}

func (db *cacheDB) Iterator() Iterator {
	panic("cacheDB.Iterator() not yet supported")
}

func (db *cacheDB) NewBatch() Batch {
	return &memBatch{db, nil}
}

// Implements `atomicSetDeleter` for Batch support.
func (db *cacheDB) Mutex() *sync.Mutex {
	return &(db.mtx)
}

// Write writes pending updates to the parent database and clears the cache.
func (db *cacheDB) Write() {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	// Optional sanity check to ensure that cacheDB is valid
	if parent, ok := db.parent.(WriteLocker); ok {
		if parent.TryWriteLock(db.lockVersion) {
			// All good!
		} else {
			panic("cacheDB.Write() failed. Did this CacheDB expire?")
		}
	}

	// We need a copy of all of the keys.
	// Not the best, but probably not a bottleneck depending.
	keys := make([]string, 0, len(db.cache))
	for key, dbValue := range db.cache {
		if dbValue.dirty {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	batch := db.parent.NewBatch()
	for _, key := range keys {
		dbValue := db.cache[key]
		if dbValue.deleted {
			batch.Delete([]byte(key))
		} else if dbValue.value == nil {
			// Skip, it already doesn't exist in parent.
		} else {
			batch.Set([]byte(key), dbValue.value)
		}
	}
	batch.Write()

	// Clear the cache
	db.cache = make(map[string]cDBValue)
}

//----------------------------------------
// To cache-wrap this cacheDB further.

func (db *cacheDB) CacheDB() CacheDB {
	return NewCacheDB(db, db.GetWriteLockVersion())
}

// If the parent parent DB implements this, (e.g. such as a cacheDB parent to a
// cacheDB child), cacheDB will call `parent.TryWriteLock()` before attempting
// to write.
type WriteLocker interface {
	GetWriteLockVersion() (lockVersion interface{})
	TryWriteLock(lockVersion interface{}) bool
}

// Implements TryWriteLocker.  Embed this in DB structs if desired.
type cwwMutex struct {
	mtx sync.Mutex
	// CONTRACT: reading/writing to `*written` should use `atomic.*`.
	// CONTRACT: replacing `written` with another *int32 should use `.mtx`.
	written *int32
}

func NewCWWMutex() cwwMutex {
	return cwwMutex{
		written: new(int32),
	}
}

func (cww *cwwMutex) GetWriteLockVersion() interface{} {
	cww.mtx.Lock()
	defer cww.mtx.Unlock()

	// `written` works as a "version" object because it gets replaced upon
	// successful TryWriteLock.
	return cww.written
}

func (cww *cwwMutex) TryWriteLock(version interface{}) bool {
	cww.mtx.Lock()
	defer cww.mtx.Unlock()

	if version != cww.written {
		return false // wrong "WriteLockVersion"
	}
	if !atomic.CompareAndSwapInt32(cww.written, 0, 1) {
		return false // already written
	}

	// New "WriteLockVersion"
	cww.written = new(int32)
	return true
}
