package db

import (
	"path"

	. "github.com/tendermint/tendermint2/common"
	"github.com/tendermint/tendermint2/config"
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
	switch config.App().GetString("DB.Backend") {
	case DBBackendMemDB:
		db := NewMemDB()
		dbs.Set(name, db)
		return db
	case DBBackendLevelDB:
		db, err := NewLevelDB(path.Join(config.App().GetString("DB.Dir"), name+".db"))
		if err != nil {
			panic(err)
		}
		dbs.Set(name, db)
		return db
	default:
		panic(Fmt("Unknown DB backend: %v", config.App().GetString("DB.Backend")))
	}
}
