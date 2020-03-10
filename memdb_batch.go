package db

import "github.com/pkg/errors"

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

var _ Batch = (*memDBBatch)(nil)

// newMemDBBatch creates a new memDBBatch
func newMemDBBatch(db *MemDB) *memDBBatch {
	return &memDBBatch{
		db:  db,
		ops: []operation{},
	}
}

func (b *memDBBatch) assertOpen() {
	if b.ops == nil {
		panic("batch has been written or closed")
	}
}

// Set implements Batch.
func (b *memDBBatch) Set(key, value []byte) {
	b.assertOpen()
	b.ops = append(b.ops, operation{opTypeSet, key, value})
}

// Delete implements Batch.
func (b *memDBBatch) Delete(key []byte) {
	b.assertOpen()
	b.ops = append(b.ops, operation{opTypeDelete, key, nil})
}

// Write implements Batch.
func (b *memDBBatch) Write() error {
	b.assertOpen()
	b.db.mtx.Lock()
	defer b.db.mtx.Unlock()

	for _, op := range b.ops {
		switch op.opType {
		case opTypeSet:
			b.db.set(op.key, op.value)
		case opTypeDelete:
			b.db.delete(op.key)
		default:
			return errors.Errorf("unknown operation type %v (%v)", op.opType, op)
		}
	}

	// Make sure batch cannot be used afterwards. Callers should still call Close(), for errors.
	b.Close()
	return nil
}

// WriteSync implements Batch.
func (b *memDBBatch) WriteSync() error {
	return b.Write()
}

// Close implements Batch.
func (b *memDBBatch) Close() {
	b.ops = nil
}
