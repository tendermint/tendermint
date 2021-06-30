package p2p_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/pkg/p2p"
)

func TestLoadNodeKey(t *testing.T) {
	filePath := filepath.Join(os.TempDir(), tmrand.Str(12)+"_peer_id.json")

	_, err := p2p.LoadNodeKeyID(filePath)
	require.True(t, os.IsNotExist(err))

	_, err = p2p.LoadOrGenNodeKeyID(filePath)
	require.NoError(t, err)

	nodeKey, err := p2p.LoadNodeKeyID(filePath)
	require.NoError(t, err)
	require.NotNil(t, nodeKey)
}

func TestLoadOrGenNodeKey(t *testing.T) {
	filePath := filepath.Join(os.TempDir(), tmrand.Str(12)+"_peer_id.json")

	nodeKey, err := p2p.LoadOrGenNodeKeyID(filePath)
	require.Nil(t, err)

	nodeKey2, err := p2p.LoadOrGenNodeKeyID(filePath)
	require.Nil(t, err)
	require.Equal(t, nodeKey, nodeKey2)
}
