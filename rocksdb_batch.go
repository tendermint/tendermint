// +build rocksdb

package db

import "github.com/tecbot/gorocksdb"

type rocksDBBatch struct {
	db    *RocksDB
	batch *gorocksdb.WriteBatch
}

var _ Batch = (*rocksDBBatch)(nil)

func newRocksDBBatch(db *RocksDB) *rocksDBBatch {
	return &rocksDBBatch{
		db:    db,
		batch: gorocksdb.NewWriteBatch(),
	}
}

func (b *rocksDBBatch) assertOpen() {
	if b.batch == nil {
		panic("batch has been written or closed")
	}
}

// Set implements Batch.
func (b *rocksDBBatch) Set(key, value []byte) {
	b.assertOpen()
	b.batch.Put(key, value)
}

// Delete implements Batch.
func (b *rocksDBBatch) Delete(key []byte) {
	b.assertOpen()
	b.batch.Delete(key)
}

// Write implements Batch.
func (b *rocksDBBatch) Write() error {
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
func (b *rocksDBBatch) WriteSync() error {
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
func (b *rocksDBBatch) Close() {
	if b.batch != nil {
		b.batch.Destroy()
		b.batch = nil
	}
}
