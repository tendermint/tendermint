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
	NewBatch() Batch

	// For debugging
	Print()
}

type Batch interface {
	Set(key, value []byte)
	Delete(key []byte)
	Write()
}

//-----------------------------------------------------------------------------

// Database types
const DBBackendMemDB = "memdb"
const DBBackendLevelDB = "leveldb"
const DBBackendLevelDB2 = "leveldb2"

func NewDB(name string, backend string, dir string) DB {
	switch backend {
	case DBBackendMemDB:
		db := NewMemDB()
		return db
	case DBBackendLevelDB:
		db, err := NewLevelDB(path.Join(dir, name+".db"))
		if err != nil {
			PanicCrisis(err)
		}
		return db
	case DBBackendLevelDB2:
		db, err := NewLevelDB2(path.Join(dir, name+".db"))
		if err != nil {
			PanicCrisis(err)
		}
		return db
	default:
		PanicSanity(Fmt("Unknown DB backend: %v", backend))
	}
	return nil
}
