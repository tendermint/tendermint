package db

import (
	"fmt"
	"path"

	"github.com/jmhodges/levigo"

	. "github.com/tendermint/go-common"
)

type LevelDB2 struct {
	db     *levigo.DB
	ro     *levigo.ReadOptions
	wo     *levigo.WriteOptions
	woSync *levigo.WriteOptions
}

func NewLevelDB2(name string) (*LevelDB2, error) {
	dbPath := path.Join(name)

	opts := levigo.NewOptions()
	opts.SetCache(levigo.NewLRUCache(1 << 30))
	opts.SetCreateIfMissing(true)
	db, err := levigo.Open(dbPath, opts)
	if err != nil {
		return nil, err
	}
	ro := levigo.NewReadOptions()
	wo := levigo.NewWriteOptions()
	woSync := levigo.NewWriteOptions()
	woSync.SetSync(true)
	database := &LevelDB2{
		db:     db,
		ro:     ro,
		wo:     wo,
		woSync: woSync,
	}
	return database, nil
}

func (db *LevelDB2) Get(key []byte) []byte {
	res, err := db.db.Get(db.ro, key)
	if err != nil {
		PanicCrisis(err)
	}
	return res
}

func (db *LevelDB2) Set(key []byte, value []byte) {
	err := db.db.Put(db.wo, key, value)
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *LevelDB2) SetSync(key []byte, value []byte) {
	err := db.db.Put(db.woSync, key, value)
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *LevelDB2) Delete(key []byte) {
	err := db.db.Delete(db.wo, key)
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *LevelDB2) DeleteSync(key []byte) {
	err := db.db.Delete(db.woSync, key)
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *LevelDB2) DB() *levigo.DB {
	return db.db
}

func (db *LevelDB2) Close() {
	db.db.Close()
	db.ro.Close()
	db.wo.Close()
	db.woSync.Close()
}

func (db *LevelDB2) Print() {
	iter := db.db.NewIterator(db.ro)
	defer iter.Close()
	for iter.Seek(nil); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()
		fmt.Printf("[%X]:\t[%X]\n", key, value)
	}
}

func (db *LevelDB2) NewBatch() Batch {
	batch := levigo.NewWriteBatch()
	return &levelDB2Batch{db, batch}
}

//--------------------------------------------------------------------------------

type levelDB2Batch struct {
	db    *LevelDB2
	batch *levigo.WriteBatch
}

func (mBatch *levelDB2Batch) Set(key, value []byte) {
	mBatch.batch.Put(key, value)
}

func (mBatch *levelDB2Batch) Delete(key []byte) {
	mBatch.batch.Delete(key)
}

func (mBatch *levelDB2Batch) Write() {
	err := mBatch.db.db.Write(mBatch.db.wo, mBatch.batch)
	if err != nil {
		PanicCrisis(err)
	}
}
