package meta_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	test "github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/pkg/meta"
)

func TestBlockIDEquals(t *testing.T) {
	var (
		blockID          = meta.BlockID{[]byte("hash"), meta.PartSetHeader{2, []byte("part_set_hash")}}
		blockIDDuplicate = meta.BlockID{[]byte("hash"), meta.PartSetHeader{2, []byte("part_set_hash")}}
		blockIDDifferent = meta.BlockID{[]byte("different_hash"), meta.PartSetHeader{2, []byte("part_set_hash")}}
		blockIDEmpty     = meta.BlockID{}
	)

	assert.True(t, blockID.Equals(blockIDDuplicate))
	assert.False(t, blockID.Equals(blockIDDifferent))
	assert.False(t, blockID.Equals(blockIDEmpty))
	assert.True(t, blockIDEmpty.Equals(blockIDEmpty))
	assert.False(t, blockIDEmpty.Equals(blockIDDifferent))
}

func TestBlockIDValidateBasic(t *testing.T) {
	validBlockID := meta.BlockID{
		Hash: bytes.HexBytes{},
		PartSetHeader: meta.PartSetHeader{
			Total: 1,
			Hash:  bytes.HexBytes{},
		},
	}

	invalidBlockID := meta.BlockID{
		Hash: []byte{0},
		PartSetHeader: meta.PartSetHeader{
			Total: 1,
			Hash:  []byte{0},
		},
	}

	testCases := []struct {
		testName             string
		blockIDHash          bytes.HexBytes
		blockIDPartSetHeader meta.PartSetHeader
		expectErr            bool
	}{
		{"Valid BlockID", validBlockID.Hash, validBlockID.PartSetHeader, false},
		{"Invalid BlockID", invalidBlockID.Hash, validBlockID.PartSetHeader, true},
		{"Invalid BlockID", validBlockID.Hash, invalidBlockID.PartSetHeader, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			blockID := meta.BlockID{
				Hash:          tc.blockIDHash,
				PartSetHeader: tc.blockIDPartSetHeader,
			}
			assert.Equal(t, tc.expectErr, blockID.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestBlockIDProtoBuf(t *testing.T) {
	blockID := test.MakeBlockIDWithHash([]byte("hash"))
	testCases := []struct {
		msg     string
		bid1    *meta.BlockID
		expPass bool
	}{
		{"success", &blockID, true},
		{"success empty", &meta.BlockID{}, true},
		{"failure BlockID nil", nil, false},
	}
	for _, tc := range testCases {
		protoBlockID := tc.bid1.ToProto()

		bi, err := meta.BlockIDFromProto(&protoBlockID)
		if tc.expPass {
			require.NoError(t, err)
			require.Equal(t, tc.bid1, bi, tc.msg)
		} else {
			require.NotEqual(t, tc.bid1, bi, tc.msg)
		}
	}
}
