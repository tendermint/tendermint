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

// Set implements Batch.
func (b *memDBBatch) Set(key, value []byte) {
	b.ops = append(b.ops, operation{opTypeSet, key, value})
}

// Delete implements Batch.
func (b *memDBBatch) Delete(key []byte) {
	b.ops = append(b.ops, operation{opTypeDelete, key, nil})
}

// Write implements Batch.
func (b *memDBBatch) Write() error {
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
