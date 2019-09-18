package db

import (
	"fmt"
	"strings"
)

type DBBackendType string

// These are valid backend types.
const (
	// GoLevelDBBackend represents goleveldb (github.com/syndtr/goleveldb - most
	// popular implementation)
	//   - pure go
	//   - stable
	GoLevelDBBackend DBBackendType = "goleveldb"
	// CLevelDBBackend represents cleveldb (uses levigo wrapper)
	//   - fast
	//   - requires gcc
	//   - use cleveldb build tag (go build -tags cleveldb)
	CLevelDBBackend DBBackendType = "cleveldb"
	// MemDBBackend represents in-memoty key value store, which is mostly used
	// for testing.
	MemDBBackend DBBackendType = "memdb"
	// FSDBBackend represents filesystem database
	//	 - EXPERIMENTAL
	//   - slow
	FSDBBackend DBBackendType = "fsdb"
	// BoltDBBackend represents bolt (uses etcd's fork of bolt -
	// github.com/etcd-io/bbolt)
	//   - EXPERIMENTAL
	//   - may be faster is some use-cases (random reads - indexer)
	//   - use boltdb build tag (go build -tags boltdb)
	BoltDBBackend DBBackendType = "boltdb"
	// RocksDBBackend represents rocksdb (uses Stumble fork github.com/stumble/gorocksdb of the tecbot wrapper)
	//   - EXPERIMENTAL
	//   - requires gcc
	//   - use rocksdb build tag (go build -tags rocksdb)
	RocksDBBackend DBBackendType = "rocksdb"
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

// NewDB creates a new database of type backend with the given name.
// NOTE: function panics if:
//   - backend is unknown (not registered)
//   - creator function, provided during registration, returns error
func NewDB(name string, backend DBBackendType, dir string) DB {
	dbCreator, ok := backends[backend]
	if !ok {
		keys := make([]string, len(backends))
		i := 0
		for k := range backends {
			keys[i] = string(k)
			i++
		}
		panic(fmt.Sprintf("Unknown db_backend %s, expected either %s", backend, strings.Join(keys, " or ")))
	}

	db, err := dbCreator(name, dir)
	if err != nil {
		panic(fmt.Sprintf("Error initializing DB: %v", err))
	}
	return db
}
