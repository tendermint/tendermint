package db

import (
	"path"

	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/config"
)

type DB interface {
	Get([]byte) []byte
	Set([]byte, []byte)
	SetSync([]byte, []byte)
	Delete([]byte)
	DeleteSync([]byte)
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
	switch Config.DB.Backend {
	case DBBackendMemDB:
		db := NewMemDB()
		dbs.Set(name, db)
		return db
	case DBBackendLevelDB:
		db, err := NewLevelDB(path.Join(Config.DB.Dir, name+".db"))
		if err != nil {
			panic(err)
		}
		dbs.Set(name, db)
		return db
	default:
		panic("Unknown DB backend")
	}
}
