package db

import "fmt"

//----------------------------------------
// Main entry

type dbBackendType string

const (
	LevelDBBackend   dbBackendType = "leveldb" // legacy, defaults to goleveldb unless +gcc
	CLevelDBBackend  dbBackendType = "cleveldb"
	GoLevelDBBackend dbBackendType = "goleveldb"
	MemDBBackend     dbBackendType = "memdb"
	FSDBBackend      dbBackendType = "fsdb" // using the filesystem naively
)

type dbCreator func(name string, dir string) (DB, error)

var backends = map[dbBackendType]dbCreator{}

func registerDBCreator(backend dbBackendType, creator dbCreator, force bool) {
	_, ok := backends[backend]
	if !force && ok {
		return
	}
	backends[backend] = creator
}

func NewDB(name string, backend dbBackendType, dir string) DB {
	db, err := backends[backend](name, dir)
	if err != nil {
		panic(fmt.Sprintf("Error initializing DB: %v", err))
	}
	return db
}
