package db

import "fmt"

//----------------------------------------
// Main entry

type DBBackendType string

const (
	LevelDBBackend   DBBackendType = "leveldb" // legacy, defaults to goleveldb unless +gcc
	CLevelDBBackend  DBBackendType = "cleveldb"
	GoLevelDBBackend DBBackendType = "goleveldb"
	MemDBBackend     DBBackendType = "memdb"
	FSDBBackend      DBBackendType = "fsdb" // using the filesystem naively
)

type dbCreator func(name string, dir string) (DB, error)

var backends = map[DBBackendType]dbCreator{}

func registerDBCreator(backend DBBackendType, creator dbCreator, force bool) {
	_, ok := backends[backend]
	if !force && ok {
		return
	}
	backends[backend] = creator
}

func NewDB(name string, backend DBBackendType, dir string) DB {
	_, ok := backends[backend]
	if !ok {
		var supportType DBBackendType
		for types := range backends{
			supportType = supportType + " or " + types
		}
		panic(fmt.Sprintf("Unknown db_backend %v , expected either%v \n", backend, supportType[3:]))
	}
	db, err := backends[backend](name, dir)
	if err != nil {
		panic(fmt.Sprintf("Error initializing DB: %v", err))
	}
	return db
}
