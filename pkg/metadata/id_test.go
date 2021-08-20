package metadata_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	test "github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/pkg/metadata"
)

func TestBlockIDEquals(t *testing.T) {
	var (
		blockID          = metadata.BlockID{[]byte("hash"), metadata.PartSetHeader{2, []byte("part_set_hash")}}
		blockIDDuplicate = metadata.BlockID{[]byte("hash"), metadata.PartSetHeader{2, []byte("part_set_hash")}}
		blockIDDifferent = metadata.BlockID{[]byte("different_hash"), metadata.PartSetHeader{2, []byte("part_set_hash")}}
		blockIDEmpty     = metadata.BlockID{}
	)

	assert.True(t, blockID.Equals(blockIDDuplicate))
	assert.False(t, blockID.Equals(blockIDDifferent))
	assert.False(t, blockID.Equals(blockIDEmpty))
	assert.True(t, blockIDEmpty.Equals(blockIDEmpty))
	assert.False(t, blockIDEmpty.Equals(blockIDDifferent))
}

func TestBlockIDValidateBasic(t *testing.T) {
	validBlockID := metadata.BlockID{
		Hash: bytes.HexBytes{},
		PartSetHeader: metadata.PartSetHeader{
			Total: 1,
			Hash:  bytes.HexBytes{},
		},
	}

	invalidBlockID := metadata.BlockID{
		Hash: []byte{0},
		PartSetHeader: metadata.PartSetHeader{
			Total: 1,
			Hash:  []byte{0},
		},
	}

	testCases := []struct {
		testName             string
		blockIDHash          bytes.HexBytes
		blockIDPartSetHeader metadata.PartSetHeader
		expectErr            bool
	}{
		{"Valid BlockID", validBlockID.Hash, validBlockID.PartSetHeader, false},
		{"Invalid BlockID", invalidBlockID.Hash, validBlockID.PartSetHeader, true},
		{"Invalid BlockID", validBlockID.Hash, invalidBlockID.PartSetHeader, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			blockID := metadata.BlockID{
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
		bid1    *metadata.BlockID
		expPass bool
	}{
		{"success", &blockID, true},
		{"success empty", &metadata.BlockID{}, true},
		{"failure BlockID nil", nil, false},
	}
	for _, tc := range testCases {
		protoBlockID := tc.bid1.ToProto()

		bi, err := metadata.BlockIDFromProto(&protoBlockID)
		if tc.expPass {
			require.NoError(t, err)
			require.Equal(t, tc.bid1, bi, tc.msg)
		} else {
			require.NotEqual(t, tc.bid1, bi, tc.msg)
		}
	}
}
