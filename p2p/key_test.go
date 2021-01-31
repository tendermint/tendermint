package p2p_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/p2p"
)

func TestNewNodeID(t *testing.T) {
	// Most tests are in TestNodeID_Validate, this just checks that it's validated.
	testcases := []struct {
		input  string
		expect p2p.NodeID
		ok     bool
	}{
		{"", "", false},
		{"foo", "", false},
		{"00112233445566778899aabbccddeeff00112233", "00112233445566778899aabbccddeeff00112233", true},
		{"00112233445566778899AABBCCDDEEFF00112233", "00112233445566778899aabbccddeeff00112233", true},
		{"00112233445566778899aabbccddeeff0011223", "", false},
		{"00112233445566778899aabbccddeeff0011223g", "", false},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			id, err := p2p.NewNodeID(tc.input)
			if !tc.ok {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, id, tc.expect)
			}
		})
	}
}

func TestNewNodeIDFromPubKey(t *testing.T) {
	privKey := ed25519.GenPrivKeyFromSecret([]byte("foo"))
	nodeID := p2p.NodeIDFromPubKey(privKey.PubKey())
	require.Equal(t, p2p.NodeID("045f5600654182cfeaccfe6cb19f0642e8a59898"), nodeID)
}

func TestNodeID_Bytes(t *testing.T) {
	testcases := []struct {
		nodeID p2p.NodeID
		expect []byte
		ok     bool
	}{
		{"", []byte{}, true},
		{"01f0", []byte{0x01, 0xf0}, true},
		{"01F0", []byte{0x01, 0xf0}, true},
		{"01F", nil, false},
		{"01g0", nil, false},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(string(tc.nodeID), func(t *testing.T) {
			bz, err := tc.nodeID.Bytes()
			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, tc.expect, bz)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestNodeID_Validate(t *testing.T) {
	testcases := []struct {
		nodeID p2p.NodeID
		ok     bool
	}{
		{"", false},
		{"00", false},
		{"00112233445566778899aabbccddeeff00112233", true},
		{"00112233445566778899aabbccddeeff001122334", false},
		{"00112233445566778899aabbccddeeffgg001122", false},
		{"00112233445566778899AABBCCDDEEFF00112233", false},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(string(tc.nodeID), func(t *testing.T) {
			err := tc.nodeID.Validate()
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestLoadOrGenNodeKey(t *testing.T) {
	filePath := filepath.Join(os.TempDir(), tmrand.Str(12)+"_peer_id.json")

	nodeKey, err := p2p.LoadOrGenNodeKey(filePath)
	require.Nil(t, err)

	nodeKey2, err := p2p.LoadOrGenNodeKey(filePath)
	require.Nil(t, err)
	require.Equal(t, nodeKey, nodeKey2)
}

func TestLoadNodeKey(t *testing.T) {
	filePath := filepath.Join(os.TempDir(), tmrand.Str(12)+"_peer_id.json")

	_, err := p2p.LoadNodeKey(filePath)
	require.True(t, os.IsNotExist(err))

	_, err = p2p.LoadOrGenNodeKey(filePath)
	require.NoError(t, err)

	nodeKey, err := p2p.LoadNodeKey(filePath)
	require.NoError(t, err)
	require.NotNil(t, nodeKey)
}

func TestNodeKeySaveAs(t *testing.T) {
	filePath := filepath.Join(os.TempDir(), tmrand.Str(12)+"_peer_id.json")
	require.NoFileExists(t, filePath)

	nodeKey := p2p.GenNodeKey()
	require.NoError(t, nodeKey.SaveAs(filePath))
	require.FileExists(t, filePath)
}
