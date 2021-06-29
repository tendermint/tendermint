package p2p_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/internal/p2p"
	tmrand "github.com/tendermint/tendermint/libs/rand"
)

func TestNodeKeySaveAs(t *testing.T) {
	filePath := filepath.Join(os.TempDir(), tmrand.Str(12)+"_peer_id.json")
	require.NoFileExists(t, filePath)

	nodeKey := p2p.GenNodeKey()
	require.NoError(t, nodeKey.SaveAs(filePath))
	require.FileExists(t, filePath)
}
