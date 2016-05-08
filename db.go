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
	default:
		PanicSanity(Fmt("Unknown DB backend: %v", backend))
	}
	return nil
}
