package cleveldb

import (
	"github.com/jmhodges/levigo"
	tmdb "github.com/tendermint/tm-db"
)

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

// Set implements Batch.
func (b *cLevelDBBatch) Set(key, value []byte) error {
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
func (b *cLevelDBBatch) Delete(key []byte) error {
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
func (b *cLevelDBBatch) Write() error {
	if b.batch == nil {
		return tmdb.ErrBatchClosed
	}
	err := b.db.db.Write(b.db.wo, b.batch)
	if err != nil {
		return err
	}
	// Make sure batch cannot be used afterwards. Callers should still call Close(), for errors.
	return b.Close()
}

// WriteSync implements Batch.
func (b *cLevelDBBatch) WriteSync() error {
	if b.batch == nil {
		return tmdb.ErrBatchClosed
	}
	err := b.db.db.Write(b.db.woSync, b.batch)
	if err != nil {
		return err
	}
	// Make sure batch cannot be used afterwards. Callers should still call Close(), for errors.
	b.Close()
	return nil
}

// Close implements Batch.
func (b *cLevelDBBatch) Close() error {
	if b.batch != nil {
		b.batch.Close()
		b.batch = nil
	}
	return nil
}
