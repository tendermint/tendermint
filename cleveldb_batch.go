// +build cleveldb

package db

import "github.com/jmhodges/levigo"

// cLevelDBBatch is a LevelDB batch.
type cLevelDBBatch struct {
	db    *CLevelDB
	batch *levigo.WriteBatch
}

func newCLevelDBBatch(db *CLevelDB) *cLevelDBBatch {
	return &cLevelDBBatch{
		db:    db,
		batch: levigo.NewWriteBatch(),
	}
}

func (b *cLevelDBBatch) assertOpen() {
	if b.batch == nil {
		panic("batch has been written or closed")
	}
}

// Set implements Batch.
func (b *cLevelDBBatch) Set(key, value []byte) {
	b.assertOpen()
	b.batch.Put(nonNilBytes(key), nonNilBytes(value))
}

// Delete implements Batch.
func (b *cLevelDBBatch) Delete(key []byte) {
	b.assertOpen()
	b.batch.Delete(nonNilBytes(key))
}

// Write implements Batch.
func (b *cLevelDBBatch) Write() error {
	b.assertOpen()
	err := b.db.db.Write(b.db.wo, b.batch)
	if err != nil {
		return err
	}
	// Make sure batch cannot be used afterwards. Callers should still call Close(), for errors.
	b.Close()
	return nil
}

// WriteSync implements Batch.
func (b *cLevelDBBatch) WriteSync() error {
	b.assertOpen()
	err := b.db.db.Write(b.db.woSync, b.batch)
	if err != nil {
		return err
	}
	// Make sure batch cannot be used afterwards. Callers should still call Close(), for errors.
	b.Close()
	return nil
}

// Close implements Batch.
func (b *cLevelDBBatch) Close() {
	if b.batch != nil {
		b.batch.Close()
		b.batch = nil
	}
}
