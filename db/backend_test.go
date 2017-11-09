package db

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	cmn "github.com/tendermint/tmlibs/common"
)

func testBackend(t *testing.T, backend string) {
	// Default
	dir, dirname := cmn.Tempdir(fmt.Sprintf("test_backend_%s_", backend))
	defer dir.Close()
	db := NewDB("testdb", backend, dirname)
	require.Nil(t, db.Get([]byte("")))
	require.Nil(t, db.Get(nil))

	// Set empty ("")
	db.Set([]byte(""), []byte(""))
	require.NotNil(t, db.Get([]byte("")))
	require.NotNil(t, db.Get(nil))
	require.Empty(t, db.Get([]byte("")))
	require.Empty(t, db.Get(nil))

	// Set empty (nil)
	db.Set([]byte(""), nil)
	require.NotNil(t, db.Get([]byte("")))
	require.NotNil(t, db.Get(nil))
	require.Empty(t, db.Get([]byte("")))
	require.Empty(t, db.Get(nil))

	// Delete
	db.Delete([]byte(""))
	require.Nil(t, db.Get([]byte("")))
	require.Nil(t, db.Get(nil))
}

func TestBackends(t *testing.T) {
	testBackend(t, CLevelDBBackendStr)
	testBackend(t, GoLevelDBBackendStr)
	testBackend(t, MemDBBackendStr)
}
