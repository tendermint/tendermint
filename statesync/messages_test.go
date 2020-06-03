package statesync

import (
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	ssproto "github.com/tendermint/tendermint/proto/statesync"
	tmproto "github.com/tendermint/tendermint/proto/types"
)

func TestValidateMsg(t *testing.T) {
	testcases := map[string]struct {
		msg   proto.Message
		valid bool
	}{
		"nil":       {nil, false},
		"unrelated": {&tmproto.Block{}, false},

		"ChunkRequest valid":    {&ssproto.ChunkRequest{Height: 1, Format: 1, Index: 1}, true},
		"ChunkRequest 0 height": {&ssproto.ChunkRequest{Height: 0, Format: 1, Index: 1}, false},
		"ChunkRequest 0 format": {&ssproto.ChunkRequest{Height: 1, Format: 0, Index: 1}, true},
		"ChunkRequest 0 chunk":  {&ssproto.ChunkRequest{Height: 1, Format: 1, Index: 0}, true},

		"ChunkResponse valid": {
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Chunk: []byte{1}},
			true},
		"ChunkResponse 0 height": {
			&ssproto.ChunkResponse{Height: 0, Format: 1, Index: 1, Chunk: []byte{1}},
			false},
		"ChunkResponse 0 format": {
			&ssproto.ChunkResponse{Height: 1, Format: 0, Index: 1, Chunk: []byte{1}},
			true},
		"ChunkResponse 0 chunk": {
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 0, Chunk: []byte{1}},
			true},
		"ChunkResponse empty body": {
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Chunk: []byte{}},
			true},
		"ChunkResponse nil body": {
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Chunk: nil},
			false},
		"ChunkResponse missing": {
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Missing: true},
			true},
		"ChunkResponse missing with empty": {
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Missing: true, Chunk: []byte{}},
			true},
		"ChunkResponse missing with body": {
			&ssproto.ChunkResponse{Height: 1, Format: 1, Index: 1, Missing: true, Chunk: []byte{1}},
			false},

		"SnapshotsRequest valid": {&ssproto.SnapshotsRequest{}, true},

		"SnapshotsResponse valid": {
			&ssproto.SnapshotsResponse{Height: 1, Format: 1, Chunks: 2, Hash: []byte{1}},
			true},
		"SnapshotsResponse 0 height": {
			&ssproto.SnapshotsResponse{Height: 0, Format: 1, Chunks: 2, Hash: []byte{1}},
			false},
		"SnapshotsResponse 0 format": {
			&ssproto.SnapshotsResponse{Height: 1, Format: 0, Chunks: 2, Hash: []byte{1}},
			true},
		"SnapshotsResponse 0 chunks": {
			&ssproto.SnapshotsResponse{Height: 1, Format: 1, Hash: []byte{1}},
			false},
		"SnapshotsResponse no hash": {
			&ssproto.SnapshotsResponse{Height: 1, Format: 1, Chunks: 2, Hash: []byte{}},
			false},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := validateMsg(tc.msg)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
