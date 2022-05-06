package factory

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/types"
)

// NodeID returns a valid NodeID based on an inputted string
func NodeID(t *testing.T, str string) types.NodeID {
	t.Helper()

	id, err := types.NewNodeID(strings.Repeat(str, 2*types.NodeIDByteLength))
	require.NoError(t, err)

	return id
}

// RandomNodeID returns a randomly generated valid NodeID
func RandomNodeID(t *testing.T) types.NodeID {
	t.Helper()

	id, err := types.NewNodeID(hex.EncodeToString(rand.Bytes(types.NodeIDByteLength)))
	require.NoError(t, err)

	return id
}
