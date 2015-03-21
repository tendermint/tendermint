package db

import (
	"fmt"
)

type MemDB struct {
	db map[string][]byte
}

func NewMemDB() *MemDB {
	database := &MemDB{db: make(map[string][]byte)}
	return database
}

func (db *MemDB) Get(key []byte) []byte {
	return db.db[string(key)]
}

func (db *MemDB) Set(key []byte, value []byte) {
	db.db[string(key)] = value
}

func (db *MemDB) SetSync(key []byte, value []byte) {
	db.db[string(key)] = value
}

func (db *MemDB) Delete(key []byte) {
	delete(db.db, string(key))
}

func (db *MemDB) DeleteSync(key []byte) {
	delete(db.db, string(key))
}

func (db *MemDB) Close() {
	db = nil
}

func (db *MemDB) Print() {
	for key, value := range db.db {
		fmt.Printf("[%X]:\t[%X]\n", []byte(key), value)
	}
}
