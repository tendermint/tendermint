package statesync_test

import (
	"encoding/hex"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func TestValidateMsg(t *testing.T) {
	testcases := map[string]struct {
		msg      proto.Message
		validMsg bool
		valid    bool
	}{
		"nil":       {nil, false, false},
		"unrelated": {&tmproto.Block{}, false, false},

		"ChunkRequest valid":    {&ssproto.ChunkRequest{Height: 1, Format: 1, Index: 1}, true, true},
		"ChunkRequest 0 height": {&ssproto.ChunkRequest{Height: 0, Format: 1, Index: 1}, true, false},
		"ChunkRequest 0 format": {&ssproto.ChunkRequest{Height: 1, Format: 0, Index: 1}, true, true},
		"ChunkRequest 0 chunk":  {&ssproto.ChunkRequest{Height: 1, Format: 1, Index: 0}, true, true},

		"ChunkResponse valid": {
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Chunk: []byte{1}},
			true,
			true,
		},
		"ChunkResponse 0 height": {
			&ssproto.ChunkResponse{Height: 0, Format: 1, Index: 1, Chunk: []byte{1}},
			true,
			false,
		},
		"ChunkResponse 0 format": {
			&ssproto.ChunkResponse{Height: 1, Format: 0, Index: 1, Chunk: []byte{1}},
			true,
			true,
		},
		"ChunkResponse 0 chunk": {
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 0, Chunk: []byte{1}},
			true,
			true,
		},
		"ChunkResponse empty body": {
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Chunk: []byte{}},
			true,
			true,
		},
		"ChunkResponse nil body": {
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Chunk: nil},
			true,
			false,
		},
		"ChunkResponse missing": {
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Missing: true},
			true,
			true,
		},
		"ChunkResponse missing with empty": {
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Missing: true, Chunk: []byte{}},
			true,
			true,
		},
		"ChunkResponse missing with body": {
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Missing: true, Chunk: []byte{1}},
			true,
			false,
		},

		"SnapshotsRequest valid": {&ssproto.SnapshotsRequest{}, true, true},

		"SnapshotsResponse valid": {
			&ssproto.SnapshotsResponse{Height: 1, Format: 1, Chunks: 2, Hash: []byte{1}},
			true,
			true,
		},
		"SnapshotsResponse 0 height": {
			&ssproto.SnapshotsResponse{Height: 0, Format: 1, Chunks: 2, Hash: []byte{1}},
			true,
			false,
		},
		"SnapshotsResponse 0 format": {
			&ssproto.SnapshotsResponse{Height: 1, Format: 0, Chunks: 2, Hash: []byte{1}},
			true,
			true,
		},
		"SnapshotsResponse 0 chunks": {
			&ssproto.SnapshotsResponse{Height: 1, Format: 1, Hash: []byte{1}},
			true,
			false,
		},
		"SnapshotsResponse no hash": {
			&ssproto.SnapshotsResponse{Height: 1, Format: 1, Chunks: 2, Hash: []byte{}},
			true,
			false,
		},
	}

	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			msg := new(ssproto.Message)

			if tc.validMsg {
				require.NoError(t, msg.Wrap(tc.msg))
			} else {
				require.Error(t, msg.Wrap(tc.msg))
			}

			if tc.valid {
				require.NoError(t, msg.Validate())
			} else {
				require.Error(t, msg.Validate())
			}
		})
	}
}

func TestStateSyncVectors(t *testing.T) {
	testCases := []struct {
		testName string
		msg      proto.Message
		expBytes string
	}{
		{
			"SnapshotsRequest",
			&ssproto.SnapshotsRequest{},
			"0a00",
		},
		{
			"SnapshotsResponse",
			&ssproto.SnapshotsResponse{
				Height:   1,
				Format:   2,
				Chunks:   3,
				Hash:     []byte("chuck hash"),
				Metadata: []byte("snapshot metadata"),
			},
			"1225080110021803220a636875636b20686173682a11736e617073686f74206d65746164617461",
		},
		{
			"ChunkRequest",
			&ssproto.ChunkRequest{
				Height: 1,
				Format: 2,
				Index:  3,
			},
			"1a06080110021803",
		},
		{
			"ChunkResponse",
			&ssproto.ChunkResponse{
				Height: 1,
				Format: 2,
				Index:  3,
				Chunk:  []byte("it's a chunk"),
			},
			"2214080110021803220c697427732061206368756e6b",
		},
		{
			"LightBlockRequest",
			&ssproto.LightBlockRequest{
				Height: 100,
			},
			"2a020864",
		},
		{
			"LightBlockResponse",
			&ssproto.LightBlockResponse{
				LightBlock: nil,
			},
			"3200",
		},
		{
			"ParamsRequest",
			&ssproto.ParamsRequest{
				Height: 9001,
			},
			"3a0308a946",
		},
		{
			"ParamsResponse",
			&ssproto.ParamsResponse{
				Height:          9001,
				ConsensusParams: types.DefaultConsensusParams().ToProto(),
			},
			"423408a946122f0a10088080c00a10ffffffffffffffffff01120e08a08d0612040880c60a188080401a090a07656432353531392200",
		},
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
