package db

import (
	"fmt"
	"path"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/opt"

	. "github.com/tendermint/go-common"
)

type LevelDB struct {
	db *leveldb.DB
}

func NewLevelDB(name string) (*LevelDB, error) {
	dbPath := path.Join(name)
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		return nil, err
	}
	database := &LevelDB{db: db}
	return database, nil
}

func (db *LevelDB) Get(key []byte) []byte {
	res, err := db.db.Get(key, nil)
	if err != nil {
		if err == errors.ErrNotFound {
			return nil
		} else {
			PanicCrisis(err)
		}
	}
	return res
}

func (db *LevelDB) Set(key []byte, value []byte) {
	err := db.db.Put(key, value, nil)
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *LevelDB) SetSync(key []byte, value []byte) {
	err := db.db.Put(key, value, &opt.WriteOptions{Sync: true})
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *LevelDB) Delete(key []byte) {
	err := db.db.Delete(key, nil)
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *LevelDB) DeleteSync(key []byte) {
	err := db.db.Delete(key, &opt.WriteOptions{Sync: true})
	if err != nil {
		PanicCrisis(err)
	}
}

func (db *LevelDB) DB() *leveldb.DB {
	return db.db
}

func (db *LevelDB) Close() {
	db.db.Close()
}

func (db *LevelDB) Print() {
	iter := db.db.NewIterator(nil, nil)
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		fmt.Printf("[%X]:\t[%X]\n", key, value)
	}
}

func (db *LevelDB) NewBatch() Batch {
	batch := new(leveldb.Batch)
	return &levelDBBatch{db, batch}
}

//--------------------------------------------------------------------------------

type levelDBBatch struct {
	db    *LevelDB
	batch *leveldb.Batch
}

func (mBatch *levelDBBatch) Set(key, value []byte) {
	mBatch.batch.Put(key, value)
}

func (mBatch *levelDBBatch) Delete(key []byte) {
	mBatch.batch.Delete(key)
}

func (mBatch *levelDBBatch) Write() {
	err := mBatch.db.db.Write(mBatch.batch, nil)
	if err != nil {
		PanicCrisis(err)
	}
}
