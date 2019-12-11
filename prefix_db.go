package db

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/pkg/errors"
)

// IteratePrefix is a convenience function for iterating over a key domain
// restricted by prefix.
func IteratePrefix(db DB, prefix []byte) (Iterator, error) {
	var start, end []byte
	if len(prefix) == 0 {
		start = nil
		end = nil
	} else {
		start = cp(prefix)
		end = cpIncr(prefix)
	}
	itr, err := db.Iterator(start, end)
	if err != nil {
		return nil, err
	}
	return itr, nil
}

/*
TODO: Make test, maybe rename.
// Like IteratePrefix but the iterator strips the prefix from the keys.
func IteratePrefixStripped(db DB, prefix []byte) Iterator {
	start, end := ...
	return newPrefixIterator(prefix, start, end, IteratePrefix(db, prefix))
}
*/

//----------------------------------------
// prefixDB

type PrefixDB struct {
	mtx    sync.Mutex
	prefix []byte
	db     DB
}

// NewPrefixDB lets you namespace multiple DBs within a single DB.
func NewPrefixDB(db DB, prefix []byte) *PrefixDB {
	return &PrefixDB{
		prefix: prefix,
		db:     db,
	}
}

// Implements atomicSetDeleter.
func (pdb *PrefixDB) Mutex() *sync.Mutex {
	return &(pdb.mtx)
}

// Implements DB.
func (pdb *PrefixDB) Get(key []byte) ([]byte, error) {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	pkey := pdb.prefixed(key)
	value, err := pdb.db.Get(pkey)
	if err != nil {
		return nil, err
	}
	return value, nil
}

// Implements DB.
func (pdb *PrefixDB) Has(key []byte) (bool, error) {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	ok, err := pdb.db.Has(pdb.prefixed(key))
	if err != nil {
		return ok, err
	}

	return ok, nil
}

// Implements DB.
func (pdb *PrefixDB) Set(key []byte, value []byte) error {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	pkey := pdb.prefixed(key)
	if err := pdb.db.Set(pkey, value); err != nil {
		return err
	}
	return nil
}

// Implements DB.
func (pdb *PrefixDB) SetSync(key []byte, value []byte) error {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	return pdb.db.SetSync(pdb.prefixed(key), value)
}

// Implements DB.
func (pdb *PrefixDB) Delete(key []byte) error {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	return pdb.db.Delete(pdb.prefixed(key))
}

// Implements DB.
func (pdb *PrefixDB) DeleteSync(key []byte) error {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	return pdb.db.DeleteSync(pdb.prefixed(key))
}

// Implements DB.
func (pdb *PrefixDB) Iterator(start, end []byte) (Iterator, error) {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	var pstart, pend []byte
	pstart = append(cp(pdb.prefix), start...)
	if end == nil {
		pend = cpIncr(pdb.prefix)
	} else {
		pend = append(cp(pdb.prefix), end...)
	}
	itr, err := pdb.db.Iterator(pstart, pend)
	if err != nil {
		return nil, err
	}
	return newPrefixIterator(
		pdb.prefix,
		start,
		end,
		itr,
	), nil
}

// Implements DB.
func (pdb *PrefixDB) ReverseIterator(start, end []byte) (Iterator, error) {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	var pstart, pend []byte
	pstart = append(cp(pdb.prefix), start...)
	if end == nil {
		pend = cpIncr(pdb.prefix)
	} else {
		pend = append(cp(pdb.prefix), end...)
	}
	ritr, err := pdb.db.ReverseIterator(pstart, pend)
	if err != nil {
		return nil, err
	}
	return newPrefixIterator(
		pdb.prefix,
		start,
		end,
		ritr,
	), nil
}

// Implements DB.
// Panics if the underlying DB is not an
// atomicSetDeleter.
func (pdb *PrefixDB) NewBatch() Batch {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	return newPrefixBatch(pdb.prefix, pdb.db.NewBatch())
}

/* NOTE: Uncomment to use memBatch instead of prefixBatch
// Implements atomicSetDeleter.
func (pdb *PrefixDB) SetNoLock(key []byte, value []byte) {
	pdb.db.(atomicSetDeleter).SetNoLock(pdb.prefixed(key), value)
}

// Implements atomicSetDeleter.
func (pdb *PrefixDB) SetNoLockSync(key []byte, value []byte) {
	pdb.db.(atomicSetDeleter).SetNoLockSync(pdb.prefixed(key), value)
}

// Implements atomicSetDeleter.
func (pdb *PrefixDB) DeleteNoLock(key []byte) {
	pdb.db.(atomicSetDeleter).DeleteNoLock(pdb.prefixed(key))
}

// Implements atomicSetDeleter.
func (pdb *PrefixDB) DeleteNoLockSync(key []byte) {
	pdb.db.(atomicSetDeleter).DeleteNoLockSync(pdb.prefixed(key))
}
*/

// Implements DB.
func (pdb *PrefixDB) Close() error {
	pdb.mtx.Lock()
	defer pdb.mtx.Unlock()

	return pdb.db.Close()
}

// Implements DB.
func (pdb *PrefixDB) Print() error {
	fmt.Printf("prefix: %X\n", pdb.prefix)

	itr, err := pdb.Iterator(nil, nil)
	if err != nil {
		return err
	}
	defer itr.Close()
	for ; itr.Valid(); err = itr.Next() {
		if err != nil {
			return errors.Wrap(err, "next")
		}
		key, err := itr.Key()
		if err != nil {
			return errors.Wrap(err, "key")
		}
		value, err := itr.Value()
		if err != nil {
			return errors.Wrap(err, "value")
		}
		fmt.Printf("[%X]:\t[%X]\n", key, value)
	}
	return nil
}

// Implements DB.
func (pdb *PrefixDB) Stats() map[string]string {
	stats := make(map[string]string)
	stats["prefixdb.prefix.string"] = string(pdb.prefix)
	stats["prefixdb.prefix.hex"] = fmt.Sprintf("%X", pdb.prefix)
	source := pdb.db.Stats()
	for key, value := range source {
		stats["prefixdb.source."+key] = value
	}
	return stats
}

func (pdb *PrefixDB) prefixed(key []byte) []byte {
	return append(cp(pdb.prefix), key...)
}

//----------------------------------------
// prefixBatch

type prefixBatch struct {
	prefix []byte
	source Batch
}

func newPrefixBatch(prefix []byte, source Batch) prefixBatch {
	return prefixBatch{
		prefix: prefix,
		source: source,
	}
}

func (pb prefixBatch) Set(key, value []byte) {
	pkey := append(cp(pb.prefix), key...)
	pb.source.Set(pkey, value)
}

func (pb prefixBatch) Delete(key []byte) {
	pkey := append(cp(pb.prefix), key...)
	pb.source.Delete(pkey)
}

func (pb prefixBatch) Write() error {
	return pb.source.Write()
}

func (pb prefixBatch) WriteSync() error {
	return pb.source.WriteSync()
}

func (pb prefixBatch) Close() {
	pb.source.Close()
}

//----------------------------------------
// prefixIterator

var _ Iterator = (*prefixIterator)(nil)

// Strips prefix while iterating from Iterator.
type prefixIterator struct {
	prefix []byte
	start  []byte
	end    []byte
	source Iterator
	valid  bool
}

func newPrefixIterator(prefix, start, end []byte, source Iterator) *prefixIterator {
	// Ignoring the error here as the iterator is invalid
	// but this is being conveyed in the below if statement
	key, _ := source.Key() //nolint:errcheck

	if !source.Valid() || !bytes.HasPrefix(key, prefix) {
		return &prefixIterator{
			prefix: prefix,
			start:  start,
			end:    end,
			source: source,
			valid:  false,
		}
	}
	return &prefixIterator{
		prefix: prefix,
		start:  start,
		end:    end,
		source: source,
		valid:  true,
	}
}

func (itr *prefixIterator) Domain() (start []byte, end []byte) {
	return itr.start, itr.end
}

func (itr *prefixIterator) Valid() bool {
	return itr.valid && itr.source.Valid()
}

func (itr *prefixIterator) Next() error {
	if !itr.valid {
		return errors.New("prefixIterator invalid; cannot call Next()")
	}
	err := itr.source.Next()
	if err != nil {
		return err
	}
	key, err := itr.source.Key()
	if err != nil {
		return err
	}
	if !itr.source.Valid() || !bytes.HasPrefix(key, itr.prefix) {
		itr.valid = false
		return nil
	}
	return nil
}

func (itr *prefixIterator) Key() (key []byte, err error) {
	if !itr.valid {
		return nil, errors.New("prefixIterator invalid; cannot call Key()")
	}
	key, err = itr.source.Key()
	if err != nil {
		return nil, err
	}
	return stripPrefix(key, itr.prefix), nil
}

func (itr *prefixIterator) Value() (value []byte, err error) {
	if !itr.valid {
		return nil, errors.New("prefixIterator invalid; cannot call Value()")
	}
	value, err = itr.source.Value()
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (itr *prefixIterator) Close() {
	itr.source.Close()
}

//----------------------------------------

func stripPrefix(key []byte, prefix []byte) (stripped []byte) {
	if len(key) < len(prefix) {
		panic("should not happen")
	}
	if !bytes.Equal(key[:len(prefix)], prefix) {
		panic("should not happn")
	}
	return key[len(prefix):]
}
