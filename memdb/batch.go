package memdb

import (
	"fmt"

	tmdb "github.com/tendermint/tm-db"
)

// memDBBatch operations
type opType int

const (
	opTypeSet opType = iota + 1
	opTypeDelete
)

type operation struct {
	opType
	key   []byte
	value []byte
}

// memDBBatch handles in-memory batching.
type memDBBatch struct {
	db  *MemDB
	ops []operation
}

var _ tmdb.Batch = (*memDBBatch)(nil)

// newMemDBBatch creates a new memDBBatch
func newMemDBBatch(db *MemDB) *memDBBatch {
	return &memDBBatch{
		db:  db,
		ops: []operation{},
	}
}

// Set implements Batch.
func (b *memDBBatch) Set(key, value []byte) error {
	if len(key) == 0 {
		return tmdb.ErrKeyEmpty
	}
	if value == nil {
		return tmdb.ErrValueNil
	}
	if b.ops == nil {
		return tmdb.ErrBatchClosed
	}
	b.ops = append(b.ops, operation{opTypeSet, key, value})
	return nil
}

// Delete implements Batch.
func (b *memDBBatch) Delete(key []byte) error {
	if len(key) == 0 {
		return tmdb.ErrKeyEmpty
	}
	if b.ops == nil {
		return tmdb.ErrBatchClosed
	}
	b.ops = append(b.ops, operation{opTypeDelete, key, nil})
	return nil
}

// Write implements Batch.
func (b *memDBBatch) Write() error {
	if b.ops == nil {
		return tmdb.ErrBatchClosed
	}
	b.db.mtx.Lock()
	defer b.db.mtx.Unlock()

	for _, op := range b.ops {
		switch op.opType {
		case opTypeSet:
			b.db.set(op.key, op.value)
		case opTypeDelete:
			b.db.delete(op.key)
		default:
			return fmt.Errorf("unknown operation type %v (%v)", op.opType, op)
		}
	}

	// Make sure batch cannot be used afterwards. Callers should still call Close(), for errors.
	return b.Close()
}

// WriteSync implements Batch.
func (b *memDBBatch) WriteSync() error {
	return b.Write()
}

// Close implements Batch.
func (b *memDBBatch) Close() error {
	b.ops = nil
	return nil
}
