package rocksdb

import (
	"github.com/tecbot/gorocksdb"
	tmdb "github.com/tendermint/tm-db"
)

type rocksDBBatch struct {
	db    *RocksDB
	batch *gorocksdb.WriteBatch
}

var _ tmdb.Batch = (*rocksDBBatch)(nil)

func newRocksDBBatch(db *RocksDB) *rocksDBBatch {
	return &rocksDBBatch{
		db:    db,
		batch: gorocksdb.NewWriteBatch(),
	}
}

// Set implements Batch.
func (b *rocksDBBatch) Set(key, value []byte) error {
	if len(key) == 0 {
		return tmdb.ErrKeyEmpty
	}
	if value == nil {
		return tmdb.ErrValueNil
	}
	if b.batch == nil {
		return tmdb.ErrBatchClosed
	}
	b.batch.Put(key, value)
	return nil
}

// Delete implements Batch.
func (b *rocksDBBatch) Delete(key []byte) error {
	if len(key) == 0 {
		return tmdb.ErrKeyEmpty
	}
	if b.batch == nil {
		return tmdb.ErrBatchClosed
	}
	b.batch.Delete(key)
	return nil
}

// Write implements Batch.
func (b *rocksDBBatch) Write() error {
	if b.batch == nil {
		return tmdb.ErrBatchClosed
	}
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
	if b.batch == nil {
		return tmdb.ErrBatchClosed
	}
	err := b.db.db.Write(b.db.woSync, b.batch)
	if err != nil {
		return err
	}
	// Make sure batch cannot be used afterwards. Callers should still call Close(), for errors.
	return b.Close()
}

// Close implements Batch.
func (b *rocksDBBatch) Close() error {
	if b.batch != nil {
		b.batch.Destroy()
		b.batch = nil
	}
	return nil
}
