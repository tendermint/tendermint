package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cmn "github.com/tendermint/tmlibs/common"
)

func cleanupDBDir(dir, name string) {
	os.RemoveAll(filepath.Join(dir, name) + ".db")
}

func testBackendGetSetDelete(t *testing.T, backend string) {
	// Default
	dir, dirname := cmn.Tempdir(fmt.Sprintf("test_backend_%s_", backend))
	defer dir.Close()
	db := NewDB("testdb", backend, dirname)

	key := []byte("abc")
	require.Nil(t, db.Get(key))

	// Set empty ("")
	db.Set(key, []byte(""))
	require.NotNil(t, db.Get(key))
	require.Empty(t, db.Get(key))

	// Set empty (nil)
	db.Set(key, nil)
	require.NotNil(t, db.Get(key))
	require.Empty(t, db.Get(key))

	// Delete
	db.Delete(key)
	require.Nil(t, db.Get(key))
}

func TestBackendsGetSetDelete(t *testing.T) {
	for dbType, _ := range backends {
		testBackendGetSetDelete(t, dbType)
	}
}

func TestBackendsNilKeys(t *testing.T) {
	// test all backends
	for dbType, creator := range backends {
		name := cmn.Fmt("test_%x", cmn.RandStr(12))
		db, err := creator(name, "")
		defer cleanupDBDir("", name)
		assert.Nil(t, err)

		panicMsg := "expecting %s.%s to panic"
		assert.Panics(t, func() { db.Get(nil) }, panicMsg, dbType, "get")
		assert.Panics(t, func() { db.Has(nil) }, panicMsg, dbType, "has")
		assert.Panics(t, func() { db.Set(nil, []byte("abc")) }, panicMsg, dbType, "set")
		assert.Panics(t, func() { db.SetSync(nil, []byte("abc")) }, panicMsg, dbType, "setsync")
		assert.Panics(t, func() { db.Delete(nil) }, panicMsg, dbType, "delete")
		assert.Panics(t, func() { db.DeleteSync(nil) }, panicMsg, dbType, "deletesync")

		db.Close()
	}
}

func TestGoLevelDBBackendStr(t *testing.T) {
	name := cmn.Fmt("test_%x", cmn.RandStr(12))
	db := NewDB(name, LevelDBBackendStr, "")
	defer cleanupDBDir("", name)

	if _, ok := backends[CLevelDBBackendStr]; !ok {
		_, ok := db.(*GoLevelDB)
		assert.True(t, ok)
	}
}
