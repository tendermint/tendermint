package db

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/google/btree"
)

const (
	// The approximate number of items and children per B-tree node. Tuned with benchmarks.
	bTreeDegree = 32

	// Size of the channel buffer between traversal goroutine and iterator. Using an unbuffered
	// channel causes two context switches per item sent, while buffering allows more work per
	// context switch. Tuned with benchmarks.
	chBufferSize = 64
)

// item is a btree.Item with byte slices as keys and values
type item struct {
	key   []byte
	value []byte
}

// Less implements btree.Item.
func (i *item) Less(other btree.Item) bool {
	// this considers nil == []byte{}, but that's ok since we handle nil endpoints
	// in iterators specially anyway
	return bytes.Compare(i.key, other.(*item).key) == -1
}

// newKey creates a new key item
func newKey(key []byte) *item {
	return &item{key: key}
}

// newPair creates a new pair item
func newPair(key, value []byte) *item {
	return &item{key: key, value: value}
}

func init() {
	registerDBCreator(MemDBBackend, func(name, dir string) (DB, error) {
		return NewMemDB(), nil
	}, false)
}

var _ DB = (*MemDB)(nil)

type MemDB struct {
	mtx   sync.Mutex
	btree *btree.BTree
}

func NewMemDB() *MemDB {
	database := &MemDB{
		btree: btree.New(bTreeDegree),
	}
	return database
}

// Implements atomicSetDeleter.
func (db *MemDB) Mutex() *sync.Mutex {
	return &(db.mtx)
}

// Implements DB.
func (db *MemDB) Get(key []byte) ([]byte, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	key = nonNilBytes(key)

	i := db.btree.Get(newKey(key))
	if i != nil {
		return i.(*item).value, nil
	}
	return nil, nil
}

// Implements DB.
func (db *MemDB) Has(key []byte) (bool, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	key = nonNilBytes(key)

	return db.btree.Has(newKey(key)), nil
}

// Implements DB.
func (db *MemDB) Set(key []byte, value []byte) error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
	return nil
}

// Implements DB.
func (db *MemDB) SetSync(key []byte, value []byte) error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
	return nil
}

// Implements atomicSetDeleter.
func (db *MemDB) SetNoLock(key []byte, value []byte) {
	db.SetNoLockSync(key, value)
}

// Implements atomicSetDeleter.
func (db *MemDB) SetNoLockSync(key []byte, value []byte) {
	key = nonNilBytes(key)
	value = nonNilBytes(value)

	db.btree.ReplaceOrInsert(newPair(key, value))
}

// Implements DB.
func (db *MemDB) Delete(key []byte) error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
	return nil
}

// Implements DB.
func (db *MemDB) DeleteSync(key []byte) error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
	return nil
}

// Implements atomicSetDeleter.
func (db *MemDB) DeleteNoLock(key []byte) {
	db.DeleteNoLockSync(key)
}

// Implements atomicSetDeleter.
func (db *MemDB) DeleteNoLockSync(key []byte) {
	key = nonNilBytes(key)

	db.btree.Delete(newKey(key))
}

// Implements DB.
func (db *MemDB) Close() error {
	// Close is a noop since for an in-memory
	// database, we don't have a destination
	// to flush contents to nor do we want
	// any data loss on invoking Close()
	// See the discussion in https://github.com/tendermint/tendermint/libs/pull/56
	return nil
}

// Implements DB.
func (db *MemDB) Print() error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.btree.Ascend(func(i btree.Item) bool {
		item := i.(*item)
		fmt.Printf("[%X]:\t[%X]\n", item.key, item.value)
		return true
	})
	return nil
}

// Implements DB.
func (db *MemDB) Stats() map[string]string {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	stats := make(map[string]string)
	stats["database.type"] = "memDB"
	stats["database.size"] = fmt.Sprintf("%d", db.btree.Len())
	return stats
}

// Implements DB.
func (db *MemDB) NewBatch() Batch {
	return &memBatch{db, nil}
}

//----------------------------------------
// Iterator

// Implements DB.
func (db *MemDB) Iterator(start, end []byte) (Iterator, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return newMemDBIterator(db.btree, start, end, false), nil
}

// Implements DB.
func (db *MemDB) ReverseIterator(start, end []byte) (Iterator, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return newMemDBIterator(db.btree, start, end, true), nil
}

type memDBIterator struct {
	ch     <-chan *item
	cancel context.CancelFunc
	item   *item
	start  []byte
	end    []byte
}

var _ Iterator = (*memDBIterator)(nil)

func newMemDBIterator(bt *btree.BTree, start []byte, end []byte, reverse bool) *memDBIterator {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *item, chBufferSize)
	iter := &memDBIterator{
		ch:     ch,
		cancel: cancel,
		start:  start,
		end:    end,
	}

	go func() {
		// Because we use [start, end) for reverse ranges, while btree uses (start, end], we need
		// the following variables to handle some reverse iteration conditions ourselves.
		var (
			skipEqual     []byte
			abortLessThan []byte
		)
		visitor := func(i btree.Item) bool {
			item := i.(*item)
			if skipEqual != nil && bytes.Equal(item.key, skipEqual) {
				skipEqual = nil
				return true
			}
			if abortLessThan != nil && bytes.Compare(item.key, abortLessThan) == -1 {
				return false
			}
			select {
			case <-ctx.Done():
				return false
			case ch <- item:
				return true
			}
		}
		s := newKey(start)
		e := newKey(end)
		switch {
		case start == nil && end == nil && !reverse:
			bt.Ascend(visitor)
		case start == nil && end == nil && reverse:
			bt.Descend(visitor)
		case end == nil && !reverse:
			// must handle this specially, since nil is considered less than anything else
			bt.AscendGreaterOrEqual(s, visitor)
		case !reverse:
			bt.AscendRange(s, e, visitor)
		case end == nil:
			// abort after start, since we use [start, end) while btree uses (start, end]
			abortLessThan = s.key
			bt.Descend(visitor)
		default:
			// skip end and abort after start, since we use [start, end) while btree uses (start, end]
			skipEqual = e.key
			abortLessThan = s.key
			bt.DescendLessOrEqual(e, visitor)
		}
		close(ch)
	}()

	// prime the iterator with the first value, if any
	if item, ok := <-ch; ok {
		iter.item = item
	}

	return iter
}

// Close implements Iterator.
func (i *memDBIterator) Close() {
	i.cancel()
	for range i.ch { // drain channel
	}
	i.item = nil
}

// Domain implements Iterator.
func (i *memDBIterator) Domain() ([]byte, []byte) {
	return i.start, i.end
}

// Valid implements Iterator.
func (i *memDBIterator) Valid() bool {
	return i.item != nil
}

// Next implements Iterator.
func (i *memDBIterator) Next() {
	item, ok := <-i.ch
	switch {
	case ok:
		i.item = item
	case i.item == nil:
		panic("called Next() on invalid iterator")
	default:
		i.item = nil
	}
}

// Error implements Iterator.
func (i *memDBIterator) Error() error {
	return nil // famous last words
}

// Key implements Iterator.
func (i *memDBIterator) Key() []byte {
	if i.item == nil {
		panic("called Key() on invalid iterator")
	}
	return i.item.key
}

// Value implements Iterator.
func (i *memDBIterator) Value() []byte {
	if i.item == nil {
		panic("called Value() on invalid iterator")
	}
	return i.item.value
}
