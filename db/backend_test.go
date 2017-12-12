package db

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cmn "github.com/tendermint/tmlibs/common"
)

func testBackendGetSetDelete(t *testing.T, backend string) {
	// Default
	dir, dirname := cmn.Tempdir(fmt.Sprintf("test_backend_%s_", backend))
	defer dir.Close()
	db := NewDB("testdb", backend, dirname)
	require.Nil(t, db.Get([]byte("")))

	// Set empty ("")
	db.Set([]byte(""), []byte(""))
	require.NotNil(t, db.Get([]byte("")))
	require.Empty(t, db.Get([]byte("")))

	// Set empty (nil)
	db.Set([]byte(""), nil)
	require.NotNil(t, db.Get([]byte("")))
	require.Empty(t, db.Get([]byte("")))

	// Delete
	db.Delete([]byte(""))
	require.Nil(t, db.Get([]byte("")))
}

func TestBackendsGetSetDelete(t *testing.T) {
	for dbType, _ := range backends {
		if dbType == "fsdb" {
			// TODO: handle
			// fsdb cant deal with length 0 keys
			continue
		}
		testBackendGetSetDelete(t, dbType)
	}
}

func assertPanics(t *testing.T, dbType, name string, fn func()) {
	defer func() {
		r := recover()
		assert.NotNil(t, r, cmn.Fmt("expecting %s.%s to panic", dbType, name))
	}()

	fn()
}

func TestBackendsNilKeys(t *testing.T) {
	// test all backends
	for dbType, creator := range backends {
		name := cmn.Fmt("test_%x", cmn.RandStr(12))
		db, err := creator(name, "")
		assert.Nil(t, err)
		defer os.RemoveAll(name)

		assertPanics(t, dbType, "get", func() { db.Get(nil) })
		assertPanics(t, dbType, "has", func() { db.Has(nil) })
		assertPanics(t, dbType, "set", func() { db.Set(nil, []byte("abc")) })
		assertPanics(t, dbType, "setsync", func() { db.SetSync(nil, []byte("abc")) })
		assertPanics(t, dbType, "delete", func() { db.Delete(nil) })
		assertPanics(t, dbType, "deletesync", func() { db.DeleteSync(nil) })

		db.Close()
	}
}

func TestLevelDBBackendStr(t *testing.T) {
	name := cmn.Fmt("test_%x", cmn.RandStr(12))
	db := NewDB(name, LevelDBBackendStr, "")
	defer os.RemoveAll(name)
	_, ok := db.(*GoLevelDB)
	assert.True(t, ok)
}
