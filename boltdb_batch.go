// +build boltdb

package db

import "go.etcd.io/bbolt"

// boltDBBatch stores operations internally and dumps them to BoltDB on Write().
type boltDBBatch struct {
	db  *BoltDB
	ops []operation
}

var _ Batch = (*boltDBBatch)(nil)

func newBoltDBBatch(db *BoltDB) *boltDBBatch {
	return &boltDBBatch{
		db:  db,
		ops: []operation{},
	}
}

func (b *boltDBBatch) assertOpen() {
	if b.ops == nil {
		panic("batch has been written or closed")
	}
}

// Set implements Batch.
func (b *boltDBBatch) Set(key, value []byte) {
	b.assertOpen()
	b.ops = append(b.ops, operation{opTypeSet, key, value})
}

// Delete implements Batch.
func (b *boltDBBatch) Delete(key []byte) {
	b.assertOpen()
	b.ops = append(b.ops, operation{opTypeDelete, key, nil})
}

// Write implements Batch.
func (b *boltDBBatch) Write() error {
	b.assertOpen()
	err := b.db.db.Batch(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(bucket)
		for _, op := range b.ops {
			key := nonEmptyKey(nonNilBytes(op.key))
			switch op.opType {
			case opTypeSet:
				if err := bkt.Put(key, op.value); err != nil {
					return err
				}
			case opTypeDelete:
				if err := bkt.Delete(key); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Make sure batch cannot be used afterwards. Callers should still call Close(), for errors.
	b.Close()
	return nil
}

// WriteSync implements Batch.
func (b *boltDBBatch) WriteSync() error {
	return b.Write()
}

// Close implements Batch.
func (b *boltDBBatch) Close() {
	b.ops = nil
}
