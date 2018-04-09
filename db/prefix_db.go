package db

import (
	"bytes"
	"fmt"
	"sync"
)

// IteratePrefix is a convenience function for iterating over a key domain
// restricted by prefix.
func IteratePrefix(db DB, prefix []byte) Iterator {
	var start, end []byte
	if len(prefix) == 0 {
		start = nil
		end = nil
	} else {
		start = cp(prefix)
		end = cpIncr(prefix)
	}
	return db.Iterator(start, end)
}

/*
TODO: Make test, maybe rename.
// Like IteratePrefix but the iterator strips the prefix from the keys.
func IteratePrefixStripped(db DB, prefix []byte) Iterator {
	return newUnprefixIterator(prefix, IteratePrefix(db, prefix))
}
*/

//----------------------------------------
// prefixDB

type prefixDB struct {
	mtx    sync.Mutex
	prefix []byte
	db     DB
}

// NewPrefixDB lets you namespace multiple DBs within a single DB.
func NewPrefixDB(db DB, prefix []byte) *prefixDB {
	return &prefixDB{
		prefix: prefix,
		db:     db,
	}
}

// Implements atomicSetDeleter.
func (pdb *prefixDB) Mutex() *sync.Mutex {
	return &(pdb.mtx)
}

// Implements DB.
func (pdb *prefixDB) Get(key []byte) []byte {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	return pdb.db.Get(pdb.prefixed(key))
}

// Implements DB.
func (pdb *prefixDB) Has(key []byte) bool {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	return pdb.db.Has(pdb.prefixed(key))
}

// Implements DB.
func (pdb *prefixDB) Set(key []byte, value []byte) {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	pdb.db.Set(pdb.prefixed(key), value)
}

// Implements DB.
func (pdb *prefixDB) SetSync(key []byte, value []byte) {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	pdb.db.SetSync(pdb.prefixed(key), value)
}

// Implements atomicSetDeleter.
func (pdb *prefixDB) SetNoLock(key []byte, value []byte) {
	pdb.db.Set(pdb.prefixed(key), value)
}

// Implements atomicSetDeleter.
func (pdb *prefixDB) SetNoLockSync(key []byte, value []byte) {
	pdb.db.SetSync(pdb.prefixed(key), value)
}

// Implements DB.
func (pdb *prefixDB) Delete(key []byte) {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	pdb.db.Delete(pdb.prefixed(key))
}

// Implements DB.
func (pdb *prefixDB) DeleteSync(key []byte) {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	pdb.db.DeleteSync(pdb.prefixed(key))
}

// Implements atomicSetDeleter.
func (pdb *prefixDB) DeleteNoLock(key []byte) {
	pdb.db.Delete(pdb.prefixed(key))
}

// Implements atomicSetDeleter.
func (pdb *prefixDB) DeleteNoLockSync(key []byte) {
	pdb.db.DeleteSync(pdb.prefixed(key))
}

// Implements DB.
func (pdb *prefixDB) Iterator(start, end []byte) Iterator {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	pstart := append(pdb.prefix, start...)
	pend := []byte(nil)
	if end != nil {
		pend = append(pdb.prefix, end...)
	}
	return newUnprefixIterator(
		pdb.prefix,
		pdb.db.Iterator(
			pstart,
			pend,
		),
	)
}

// Implements DB.
func (pdb *prefixDB) ReverseIterator(start, end []byte) Iterator {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	pstart := []byte(nil)
	if start != nil {
		pstart = append(pdb.prefix, start...)
	}
	pend := []byte(nil)
	if end != nil {
		pend = append(pdb.prefix, end...)
	}
	return newUnprefixIterator(
		pdb.prefix,
		pdb.db.ReverseIterator(
			pstart,
			pend,
		),
	)
}

// Implements DB.
func (pdb *prefixDB) NewBatch() Batch {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	return &memBatch{pdb, nil}
}

// Implements DB.
func (pdb *prefixDB) Close() {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	pdb.db.Close()
}

// Implements DB.
func (pdb *prefixDB) Print() {
	fmt.Printf("prefix: %X\n", pdb.prefix)

	itr := pdb.Iterator(nil, nil)
	defer itr.Close()
	for ; itr.Valid(); itr.Next() {
		key := itr.Key()
		value := itr.Value()
		fmt.Printf("[%X]:\t[%X]\n", key, value)
	}
}

// Implements DB.
func (pdb *prefixDB) Stats() map[string]string {
	stats := make(map[string]string)
	stats["prefixdb.prefix.string"] = string(pdb.prefix)
	stats["prefixdb.prefix.hex"] = fmt.Sprintf("%X", pdb.prefix)
	source := pdb.db.Stats()
	for key, value := range source {
		stats["prefixdb.source."+key] = value
	}
	return stats
}

func (pdb *prefixDB) prefixed(key []byte) []byte {
	return append(pdb.prefix, key...)
}

//----------------------------------------

// Strips prefix while iterating from Iterator.
type unprefixIterator struct {
	prefix []byte
	source Iterator
}

func newUnprefixIterator(prefix []byte, source Iterator) unprefixIterator {
	return unprefixIterator{
		prefix: prefix,
		source: source,
	}
}

func (itr unprefixIterator) Domain() (start []byte, end []byte) {
	start, end = itr.source.Domain()
	if len(start) > 0 {
		start = stripPrefix(start, itr.prefix)
	}
	if len(end) > 0 {
		end = stripPrefix(end, itr.prefix)
	}
	return
}

func (itr unprefixIterator) Valid() bool {
	return itr.source.Valid()
}

func (itr unprefixIterator) Next() {
	itr.source.Next()
}

func (itr unprefixIterator) Key() (key []byte) {
	return stripPrefix(itr.source.Key(), itr.prefix)
}

func (itr unprefixIterator) Value() (value []byte) {
	return itr.source.Value()
}

func (itr unprefixIterator) Close() {
	itr.source.Close()
}

//----------------------------------------

func stripPrefix(key []byte, prefix []byte) (stripped []byte) {
	if len(key) < len(prefix) {
		panic("should not happen")
	}
	if !bytes.Equal(key[:len(prefix)], prefix) {
		panic("should not happne")
	}
	return key[len(prefix):]
}
