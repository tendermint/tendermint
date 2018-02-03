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

func testBackendGetSetDelete(t *testing.T, backend DBBackendType) {
	// Default
	dir, dirname := cmn.Tempdir(fmt.Sprintf("test_backend_%s_", backend))
	defer dir.Close()
	db := NewDB("testdb", backend, dirname)

	// A nonexistent key should return nil, even if the key is empty
	require.Nil(t, db.Get([]byte("")))

	// A nonexistent key should return nil, even if the key is nil
	require.Nil(t, db.Get(nil))

	// A nonexistent key should return nil.
	key := []byte("abc")
	require.Nil(t, db.Get(key))

	// Set empty value.
	db.Set(key, []byte(""))
	require.NotNil(t, db.Get(key))
	require.Empty(t, db.Get(key))

	// Set nil value.
	db.Set(key, nil)
	require.NotNil(t, db.Get(key))
	require.Empty(t, db.Get(key))

	// Delete.
	db.Delete(key)
	require.Nil(t, db.Get(key))
}

func TestBackendsGetSetDelete(t *testing.T) {
	for dbType, _ := range backends {
		testBackendGetSetDelete(t, dbType)
	}
}

func withDB(t *testing.T, creator dbCreator, fn func(DB)) {
	name := cmn.Fmt("test_%x", cmn.RandStr(12))
	db, err := creator(name, "")
	defer cleanupDBDir("", name)
	assert.Nil(t, err)
	fn(db)
	db.Close()
}

func TestBackendsNilKeys(t *testing.T) {

	// Test all backends.
	for dbType, creator := range backends {
		withDB(t, creator, func(db DB) {
			t.Run(fmt.Sprintf("Testing %s", dbType), func(t *testing.T) {

				// Nil keys are treated as the empty key for most operations.
				expect := func(key, value []byte) {
					if len(key) == 0 { // nil or empty
						assert.Equal(t, db.Get(nil), db.Get([]byte("")))
						assert.Equal(t, db.Has(nil), db.Has([]byte("")))
					}
					assert.Equal(t, db.Get(key), value)
					assert.Equal(t, db.Has(key), value != nil)
				}

				// Not set
				expect(nil, nil)

				// Set nil value
				db.Set(nil, nil)
				expect(nil, []byte(""))

				// Set empty value
				db.Set(nil, []byte(""))
				expect(nil, []byte(""))

				// Set nil, Delete nil
				db.Set(nil, []byte("abc"))
				expect(nil, []byte("abc"))
				db.Delete(nil)
				expect(nil, nil)

				// Set nil, Delete empty
				db.Set(nil, []byte("abc"))
				expect(nil, []byte("abc"))
				db.Delete([]byte(""))
				expect(nil, nil)

				// Set empty, Delete nil
				db.Set([]byte(""), []byte("abc"))
				expect(nil, []byte("abc"))
				db.Delete(nil)
				expect(nil, nil)

				// Set empty, Delete empty
				db.Set([]byte(""), []byte("abc"))
				expect(nil, []byte("abc"))
				db.Delete([]byte(""))
				expect(nil, nil)

				// SetSync nil, DeleteSync nil
				db.SetSync(nil, []byte("abc"))
				expect(nil, []byte("abc"))
				db.DeleteSync(nil)
				expect(nil, nil)

				// SetSync nil, DeleteSync empty
				db.SetSync(nil, []byte("abc"))
				expect(nil, []byte("abc"))
				db.DeleteSync([]byte(""))
				expect(nil, nil)

				// SetSync empty, DeleteSync nil
				db.SetSync([]byte(""), []byte("abc"))
				expect(nil, []byte("abc"))
				db.DeleteSync(nil)
				expect(nil, nil)

				// SetSync empty, DeleteSync empty
				db.SetSync([]byte(""), []byte("abc"))
				expect(nil, []byte("abc"))
				db.DeleteSync([]byte(""))
				expect(nil, nil)
			})
		})
	}
}

func TestGoLevelDBBackend(t *testing.T) {
	name := cmn.Fmt("test_%x", cmn.RandStr(12))
	db := NewDB(name, GoLevelDBBackend, "")
	defer cleanupDBDir("", name)

	_, ok := db.(*GoLevelDB)
	assert.True(t, ok)
}
