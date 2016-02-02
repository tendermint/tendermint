package db

import (
	"path"

	. "github.com/tendermint/go-common"
)

type DB interface {
	Get([]byte) []byte
	Set([]byte, []byte)
	SetSync([]byte, []byte)
	Delete([]byte)
	DeleteSync([]byte)
	Close()

	// For debugging
	Print()
}

//-----------------------------------------------------------------------------

// Database types
const DBBackendMemDB = "memdb"
const DBBackendLevelDB = "leveldb"

var dbs = NewCMap()

func GetDB(name string) DB {
	db := dbs.Get(name)
	if db != nil {
		return db.(DB)
	}
	switch config.GetString("db_backend") {
	case DBBackendMemDB:
		db := NewMemDB()
		dbs.Set(name, db)
		return db
	case DBBackendLevelDB:
		db, err := NewLevelDB(path.Join(config.GetString("db_dir"), name+".db"))
		if err != nil {
			PanicCrisis(err)
		}
		dbs.Set(name, db)
		return db
	default:
		PanicSanity(Fmt("Unknown DB backend: %v", config.GetString("db_backend")))
	}
	return nil
}
