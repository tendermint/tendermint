// +build rocksdb

package db

import "github.com/tecbot/gorocksdb"

type rocksDBBatch struct {
	db    *RocksDB
	batch *gorocksdb.WriteBatch
}

var _ Batch = (*rocksDBBatch)(nil)

// Set implements Batch.
func (mBatch *rocksDBBatch) Set(key, value []byte) {
	mBatch.batch.Put(key, value)
}

// Delete implements Batch.
func (mBatch *rocksDBBatch) Delete(key []byte) {
	mBatch.batch.Delete(key)
}

// Write implements Batch.
func (mBatch *rocksDBBatch) Write() error {
	err := mBatch.db.db.Write(mBatch.db.wo, mBatch.batch)
	if err != nil {
		return err
	}
	return nil
}

// WriteSync mplements Batch.
func (mBatch *rocksDBBatch) WriteSync() error {
	err := mBatch.db.db.Write(mBatch.db.woSync, mBatch.batch)
	if err != nil {
		return err
	}
	return nil
}

// Close implements Batch.
func (mBatch *rocksDBBatch) Close() {
	mBatch.batch.Destroy()
}
