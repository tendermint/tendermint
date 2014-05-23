package merkle

import (
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"path"
)

type LDBDatabase struct {
	db *leveldb.DB
}

func NewLDBDatabase(name string) (*LDBDatabase, error) {
	dbPath := path.Join(name)
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		return nil, err
	}
	database := &LDBDatabase{db: db}
	return database, nil
}

func (db *LDBDatabase) Put(key []byte, value []byte) {
	err := db.db.Put(key, value, nil)
	if err != nil { panic(err) }
}

func (db *LDBDatabase) Get(key []byte) ([]byte) {
	res, err := db.db.Get(key, nil)
    if err != nil { panic(err) }
    return res
}

func (db *LDBDatabase) Delete(key []byte) error {
	return db.db.Delete(key, nil)
}

func (db *LDBDatabase) Db() *leveldb.DB {
	return db.db
}

func (db *LDBDatabase) Close() {
	db.db.Close()
}

func (db *LDBDatabase) Print() {
	iter := db.db.NewIterator(nil, nil)
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		fmt.Printf("%x(%d): %v ", key, len(key), value)
	}
}

