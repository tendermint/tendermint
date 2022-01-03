package types_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/types"
)

func mustTempPath(t *testing.T, name string) string {
	t.Helper()

	dir, err := os.MkdirTemp("", t.Name()+"*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, name)
}

func TestLoadOrGenNodeKey(t *testing.T) {
	filePath := mustTempPath(t, "peer_id.json")

	nodeKey, err := types.LoadOrGenNodeKey(filePath)
	require.Nil(t, err)

	nodeKey2, err := types.LoadOrGenNodeKey(filePath)
	require.Nil(t, err)
	require.Equal(t, nodeKey, nodeKey2)
}

func TestLoadNodeKey(t *testing.T) {
	filePath := mustTempPath(t, "peer_id.json")

	_, err := types.LoadNodeKey(filePath)
	require.True(t, os.IsNotExist(err))

	_, err = types.LoadOrGenNodeKey(filePath)
	require.NoError(t, err)

	nodeKey, err := types.LoadNodeKey(filePath)
	require.NoError(t, err)
	require.NotNil(t, nodeKey)
}

func TestNodeKeySaveAs(t *testing.T) {
	filePath := mustTempPath(t, "peer_id.json")
	require.NoFileExists(t, filePath)

	nodeKey := types.GenNodeKey()
	require.NoError(t, nodeKey.SaveAs(filePath))
	require.FileExists(t, filePath)
}
