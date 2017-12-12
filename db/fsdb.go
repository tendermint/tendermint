package db

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
)

const (
	keyPerm = os.FileMode(0600)
	dirPerm = os.FileMode(0700)
)

func init() {
	registerDBCreator(FSDBBackendStr, func(name string, dir string) (DB, error) {
		dbPath := filepath.Join(dir, name+".db")
		return NewFSDB(dbPath), nil
	}, false)
}

// It's slow.
type FSDB struct {
	mtx sync.Mutex
	dir string
}

func NewFSDB(dir string) *FSDB {
	err := os.MkdirAll(dir, dirPerm)
	if err != nil {
		panic(errors.Wrap(err, "Creating FSDB dir "+dir))
	}
	database := &FSDB{
		dir: dir,
	}
	return database
}

func (db *FSDB) Get(key []byte) []byte {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	path := db.nameToPath(key)
	value, err := read(path)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		panic(errors.Wrap(err, fmt.Sprintf("Getting key %s (0x%X)", string(key), key)))
	}
	return value
}

func (db *FSDB) Has(key []byte) bool {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	path := db.nameToPath(key)
	_, err := read(path)
	if os.IsNotExist(err) {
		return false
	} else if err != nil {
		panic(errors.Wrap(err, fmt.Sprintf("Getting key %s (0x%X)", string(key), key)))
	}
	return true
}

func (db *FSDB) Set(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
}

func (db *FSDB) SetSync(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
}

// NOTE: Implements atomicSetDeleter.
func (db *FSDB) SetNoLock(key []byte, value []byte) {
	if value == nil {
		value = []byte{}
	}
	path := db.nameToPath(key)
	err := write(path, value)
	if err != nil {
		panic(errors.Wrap(err, fmt.Sprintf("Setting key %s (0x%X)", string(key), key)))
	}
}

func (db *FSDB) Delete(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
}

func (db *FSDB) DeleteSync(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
}

// NOTE: Implements atomicSetDeleter.
func (db *FSDB) DeleteNoLock(key []byte) {
	err := remove(string(key))
	if os.IsNotExist(err) {
		return
	} else if err != nil {
		panic(errors.Wrap(err, fmt.Sprintf("Removing key %s (0x%X)", string(key), key)))
	}
}

func (db *FSDB) Close() {
	// Nothing to do.
}

func (db *FSDB) Print() {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	panic("FSDB.Print not yet implemented")
}

func (db *FSDB) Stats() map[string]string {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	panic("FSDB.Stats not yet implemented")
}

func (db *FSDB) NewBatch() Batch {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	// Not sure we would ever want to try...
	// It doesn't seem easy for general filesystems.
	panic("FSDB.NewBatch not yet implemented")
}

func (db *FSDB) Mutex() *sync.Mutex {
	return &(db.mtx)
}

func (db *FSDB) Iterator(start, end []byte) Iterator {
	/*
		XXX
		it := newMemDBIterator()
		it.db = db
		it.cur = 0

		db.mtx.Lock()
		defer db.mtx.Unlock()

		// We need a copy of all of the keys.
		// Not the best, but probably not a bottleneck depending.
		keys, err := list(db.dir)
		if err != nil {
			panic(errors.Wrap(err, fmt.Sprintf("Listing keys in %s", db.dir)))
		}
		sort.Strings(keys)
		it.keys = keys
		return it
	*/
	return nil
}

func (db *FSDB) ReverseIterator(start, end []byte) Iterator {
	// XXX
	return nil
}

func (db *FSDB) nameToPath(name []byte) string {
	n := url.PathEscape(string(name))
	return path.Join(db.dir, n)
}

// Read some bytes to a file.
// CONTRACT: returns os errors directly without wrapping.
func read(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	d, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// Write some bytes from a file.
// CONTRACT: returns os errors directly without wrapping.
func write(path string, d []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, keyPerm)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(d)
	if err != nil {
		return err
	}
	err = f.Sync()
	return err
}

// Remove a file.
// CONTRACT: returns os errors directly without wrapping.
func remove(path string) error {
	return os.Remove(path)
}

// List files of a path.
// Paths will NOT include dir as the prefix.
// CONTRACT: returns os errors directly without wrapping.
func list(dirPath string) (paths []string, err error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	names, err := dir.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	for i, name := range names {
		n, err := url.PathUnescape(name)
		if err != nil {
			return nil, fmt.Errorf("Failed to unescape %s while listing", name)
		}
		names[i] = n
	}
	return names, nil
}
