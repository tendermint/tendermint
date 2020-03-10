package db

import (
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

type goLevelDBBatch struct {
	db    *GoLevelDB
	batch *leveldb.Batch
}

var _ Batch = (*goLevelDBBatch)(nil)

func newGoLevelDBBatch(db *GoLevelDB) *goLevelDBBatch {
	return &goLevelDBBatch{
		db:    db,
		batch: new(leveldb.Batch),
	}
}

func (b *goLevelDBBatch) assertOpen() {
	if b.batch == nil {
		panic("batch has been written or closed")
	}
}

// Set implements Batch.
func (b *goLevelDBBatch) Set(key, value []byte) {
	b.assertOpen()
	b.batch.Put(key, value)
}

// Delete implements Batch.
func (b *goLevelDBBatch) Delete(key []byte) {
	b.assertOpen()
	b.batch.Delete(key)
}

// Write implements Batch.
func (b *goLevelDBBatch) Write() error {
	return b.write(false)
}

// WriteSync implements Batch.
func (b *goLevelDBBatch) WriteSync() error {
	return b.write(true)
}

func (b *goLevelDBBatch) write(sync bool) error {
	b.assertOpen()
	err := b.db.db.Write(b.batch, &opt.WriteOptions{Sync: sync})
	if err != nil {
		return err
	}
	// Make sure batch cannot be used afterwards. Callers should still call Close(), for errors.
	b.Close()
	return nil
}

// Close implements Batch.
func (b *goLevelDBBatch) Close() {
	if b.batch != nil {
		b.batch.Reset()
		b.batch = nil
	}
}
