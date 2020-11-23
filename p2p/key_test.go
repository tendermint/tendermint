package p2p

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	tmrand "github.com/tendermint/tendermint/libs/rand"
)

func TestLoadOrGenNodeKey(t *testing.T) {
	filePath := filepath.Join(os.TempDir(), tmrand.Str(12)+"_peer_id.json")

	nodeKey, err := LoadOrGenNodeKey(filePath)
	assert.Nil(t, err)

	nodeKey2, err := LoadOrGenNodeKey(filePath)
	assert.Nil(t, err)

	assert.Equal(t, nodeKey, nodeKey2)
}

func TestLoadNodeKey(t *testing.T) {
	filePath := filepath.Join(os.TempDir(), tmrand.Str(12)+"_peer_id.json")

	_, err := LoadNodeKey(filePath)
	assert.True(t, os.IsNotExist(err))

	_, err = LoadOrGenNodeKey(filePath)
	require.NoError(t, err)

	nodeKey, err := LoadNodeKey(filePath)
	assert.NoError(t, err)
	assert.NotNil(t, nodeKey)
}

func TestNodeKeySaveAs(t *testing.T) {
	filePath := filepath.Join(os.TempDir(), tmrand.Str(12)+"_peer_id.json")

	assert.NoFileExists(t, filePath)

	privKey := ed25519.GenPrivKey()
	nodeKey := &NodeKey{
		PrivKey: privKey,
	}
	err := nodeKey.SaveAs(filePath)
	assert.NoError(t, err)
	assert.FileExists(t, filePath)
}
