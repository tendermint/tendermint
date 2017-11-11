package db

import . "github.com/tendermint/tmlibs/common"

type DB interface {
	Get([]byte) []byte // NOTE: returns nil iff never set or deleted.
	Set([]byte, []byte)
	SetSync([]byte, []byte)
	Delete([]byte)
	DeleteSync([]byte)
	Close()
	NewBatch() Batch
	Iterator() Iterator

	// For debugging
	Print()

	// Stats returns a map of property values for all keys and the size of the cache.
	Stats() map[string]string

	// CacheDB wraps the DB w/ a cache.
	CacheDB() CacheDB
}

type CacheDB interface {
	DB
	Write() // Write to the underlying DB
}

type Batch interface {
	Set(key, value []byte)
	Delete(key []byte)
	Write()
}

/*
	Usage:

	for itr.Seek(mykey); itr.Valid(); itr.Next() {
		k, v := itr.Key(); itr.Value()
		....
	}
*/
type Iterator interface {

	// Seek moves the iterator the position of the key given or, if the key
	// doesn't exist, the next key that does exist in the database. If the key
	// doesn't exist, and there is no next key, the Iterator becomes invalid.
	Seek(key []byte)

	// Valid returns false only when an Iterator has iterated past either the
	// first or the last key in the database.
	Valid() bool

	// Next moves the iterator to the next sequential key in the database, as
	// defined by the Comparator in the ReadOptions used to create this Iterator.
	//
	// If Valid returns false, this method will panic.
	Next()

	// Prev moves the iterator to the previous sequential key in the database, as
	// defined by the Comparator in the ReadOptions used to create this Iterator.
	//
	// If Valid returns false, this method will panic.
	Prev()

	// Key returns the key of the cursor.
	//
	// If Valid returns false, this method will panic.
	Key() []byte

	// Value returns the key of the cursor.
	//
	// If Valid returns false, this method will panic.
	Value() []byte

	// GetError returns an IteratorError from LevelDB if it had one during
	// iteration.
	//
	// This method is safe to call when Valid returns false.
	GetError() error

	// Close deallocates the given Iterator.
	Close()
}

//-----------------------------------------------------------------------------
// Main entry

const (
	LevelDBBackendStr   = "leveldb" // legacy, defaults to goleveldb.
	CLevelDBBackendStr  = "cleveldb"
	GoLevelDBBackendStr = "goleveldb"
	MemDBBackendStr     = "memdb"
	FSDBBackendStr      = "fsdb" // using the filesystem naively
)

type dbCreator func(name string, dir string) (DB, error)

var backends = map[string]dbCreator{}

func registerDBCreator(backend string, creator dbCreator, force bool) {
	_, ok := backends[backend]
	if !force && ok {
		return
	}
	backends[backend] = creator
}

func NewDB(name string, backend string, dir string) DB {
	db, err := backends[backend](name, dir)
	if err != nil {
		PanicSanity(Fmt("Error initializing DB: %v", err))
	}
	return db
}
