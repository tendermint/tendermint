package statesync

import (
	"encoding/hex"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
)

//nolint:lll // ignore line length
func TestStateSyncVectors(t *testing.T) {
	testCases := []struct {
		testName string
		msg      proto.Message
		expBytes string
	}{
		{"SnapshotsRequest", &ssproto.SnapshotsRequest{}, "0a00"},
		{"SnapshotsResponse", &ssproto.SnapshotsResponse{Height: 1, Format: 2, Chunks: 3, Hash: []byte("chuck hash"), Metadata: []byte("snapshot metadata")}, "1225080110021803220a636875636b20686173682a11736e617073686f74206d65746164617461"},
		{"ChunkRequest", &ssproto.ChunkRequest{Height: 1, Format: 2, Index: 3}, "1a06080110021803"},
		{"ChunkResponse", &ssproto.ChunkResponse{Height: 1, Format: 2, Index: 3, Chunk: []byte("it's a chunk")}, "2214080110021803220c697427732061206368756e6b"},
	}

	for _, tc := range testCases {
		tc := tc

		msg := new(ssproto.Message)
		require.NoError(t, msg.Wrap(tc.msg))

		bz, err := msg.Marshal()
		require.NoError(t, err)
		require.Equal(t, tc.expBytes, hex.EncodeToString(bz), tc.testName)
	}
}
