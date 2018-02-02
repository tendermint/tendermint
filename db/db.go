package db

import "fmt"

//----------------------------------------
// Main entry

type DbBackendType string

const (
	LevelDBBackend   DbBackendType = "leveldb" // legacy, defaults to goleveldb unless +gcc
	CLevelDBBackend  DbBackendType = "cleveldb"
	GoLevelDBBackend DbBackendType = "goleveldb"
	MemDBBackend     DbBackendType = "memdb"
	FSDBBackend      DbBackendType = "fsdb" // using the filesystem naively
)

type dbCreator func(name string, dir string) (DB, error)

var backends = map[DbBackendType]dbCreator{}

func registerDBCreator(backend DbBackendType, creator dbCreator, force bool) {
	_, ok := backends[backend]
	if !force && ok {
		return
	}
	backends[backend] = creator
}

func NewDB(name string, backend DbBackendType, dir string) DB {
	db, err := backends[backend](name, dir)
	if err != nil {
		panic(fmt.Sprintf("Error initializing DB: %v", err))
	}
	return db
}
