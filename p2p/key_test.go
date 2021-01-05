package p2p

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	tmrand "github.com/tendermint/tendermint/libs/rand"
)

func TestLoadOrGenNodeKey(t *testing.T) {
	filePath := filepath.Join(os.TempDir(), tmrand.Str(12)+"_peer_id.json")

	nodeKey, err := LoadOrGenNodeKey(filePath)
	require.Nil(t, err)

	nodeKey2, err := LoadOrGenNodeKey(filePath)
	require.Nil(t, err)
	require.Equal(t, nodeKey, nodeKey2)
}

func TestLoadNodeKey(t *testing.T) {
	filePath := filepath.Join(os.TempDir(), tmrand.Str(12)+"_peer_id.json")

	_, err := LoadNodeKey(filePath)
	require.True(t, os.IsNotExist(err))

	_, err = LoadOrGenNodeKey(filePath)
	require.NoError(t, err)

	nodeKey, err := LoadNodeKey(filePath)
	require.NoError(t, err)
	require.NotNil(t, nodeKey)
}

func TestNodeKeySaveAs(t *testing.T) {
	filePath := filepath.Join(os.TempDir(), tmrand.Str(12)+"_peer_id.json")
	require.NoFileExists(t, filePath)

	nodeKey := GenNodeKey()
	require.NoError(t, nodeKey.SaveAs(filePath))
	require.FileExists(t, filePath)
}
