package statesync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapshotsRequestMessage_ValidateBasic(t *testing.T) {
	testcases := map[string]struct {
		msg   *snapshotsRequestMessage
		valid bool
	}{
		"nil":   {nil, false},
		"valid": {&snapshotsRequestMessage{}, true},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestSnapshotsResponseMessage_ValidateBasic(t *testing.T) {
	hashes := [][]byte{{1, 2}, {3, 4}}
	testcases := map[string]struct {
		msg   *snapshotsResponseMessage
		valid bool
	}{
		"nil":       {nil, false},
		"valid":     {&snapshotsResponseMessage{Height: 1, Format: 1, ChunkHashes: hashes}, true},
		"0 height":  {&snapshotsResponseMessage{Height: 0, Format: 1, ChunkHashes: hashes}, false},
		"0 format":  {&snapshotsResponseMessage{Height: 1, Format: 0, ChunkHashes: hashes}, true},
		"no chunks": {&snapshotsResponseMessage{Height: 1, Format: 1}, false},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestChunkRequestMessage_ValidateBasic(t *testing.T) {
	testcases := map[string]struct {
		msg   *chunkRequestMessage
		valid bool
	}{
		"nil":      {nil, false},
		"valid":    {&chunkRequestMessage{Height: 1, Format: 1, Index: 1}, true},
		"0 height": {&chunkRequestMessage{Height: 0, Format: 1, Index: 1}, false},
		"0 format": {&chunkRequestMessage{Height: 1, Format: 0, Index: 1}, true},
		"0 chunk":  {&chunkRequestMessage{Height: 1, Format: 1, Index: 0}, true},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestChunkResponseMessage_ValidateBasic(t *testing.T) {
	testcases := map[string]struct {
		msg   *chunkResponseMessage
		valid bool
	}{
		"nil message":        {nil, false},
		"valid":              {&chunkResponseMessage{Height: 1, Format: 1, Index: 1, Body: []byte{1}}, true},
		"0 height":           {&chunkResponseMessage{Height: 0, Format: 1, Index: 1, Body: []byte{1}}, false},
		"0 format":           {&chunkResponseMessage{Height: 1, Format: 0, Index: 1, Body: []byte{1}}, true},
		"0 chunk":            {&chunkResponseMessage{Height: 1, Format: 1, Index: 0, Body: []byte{1}}, true},
		"empty body":         {&chunkResponseMessage{Height: 1, Format: 1, Index: 1, Body: []byte{}}, true},
		"nil body":           {&chunkResponseMessage{Height: 1, Format: 1, Index: 1, Body: nil}, false},
		"missing":            {&chunkResponseMessage{Height: 1, Format: 1, Index: 1, Missing: true}, true},
		"missing with empty": {&chunkResponseMessage{Height: 1, Format: 1, Index: 1, Missing: true, Body: []byte{}}, true},
		"missing with body":  {&chunkResponseMessage{Height: 1, Format: 1, Index: 1, Missing: true, Body: []byte{1}}, false},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
