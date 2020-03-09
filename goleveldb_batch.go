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

// Set implements Batch.
func (b *goLevelDBBatch) Set(key, value []byte) {
	b.batch.Put(key, value)
}

// Delete implements Batch.
func (b *goLevelDBBatch) Delete(key []byte) {
	b.batch.Delete(key)
}

// Write implements Batch.
func (b *goLevelDBBatch) Write() error {
	err := b.db.db.Write(b.batch, &opt.WriteOptions{Sync: false})
	if err != nil {
		return err
	}
	return nil
}

// WriteSync implements Batch.
func (b *goLevelDBBatch) WriteSync() error {
	err := b.db.db.Write(b.batch, &opt.WriteOptions{Sync: true})
	if err != nil {
		return err
	}
	return nil
}

// Close implements Batch.
func (b *goLevelDBBatch) Close() {
	b.batch.Reset()
}
