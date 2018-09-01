package types

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	abci "github.com/tendermint/tendermint/abci/types"
)

func newConsensusParams(txsBytes, partSize int) ConsensusParams {
	return ConsensusParams{
		BlockSize:   BlockSize{MaxBytes: txsBytes},
		BlockGossip: BlockGossip{BlockPartSizeBytes: partSize},
	}
}

func TestConsensusParamsValidation(t *testing.T) {
	testCases := []struct {
		params ConsensusParams
		valid  bool
	}{
		{newConsensusParams(1, 1), true},
		{newConsensusParams(1, 0), false},
		{newConsensusParams(0, 1), false},
		{newConsensusParams(0, 0), false},
		{newConsensusParams(0, 10), false},
		{newConsensusParams(10, -1), false},
		{newConsensusParams(47*1024*1024, 400), true},
		{newConsensusParams(10, 400), true},
		{newConsensusParams(100*1024*1024, 400), true},
		{newConsensusParams(101*1024*1024, 400), false},
		{newConsensusParams(1024*1024*1024, 400), false},
	}
	for _, tc := range testCases {
		if tc.valid {
			assert.NoError(t, tc.params.Validate(), "expected no error for valid params")
		} else {
			assert.Error(t, tc.params.Validate(), "expected error for non valid params")
		}
	}
}

func makeParams(txsBytes, blockGas, txBytes, txGas, partSize int) ConsensusParams {
	return ConsensusParams{
		BlockSize: BlockSize{
			MaxBytes: txsBytes,
			MaxGas:   int64(blockGas),
		},
		TxSize: TxSize{
			MaxBytes: txBytes,
			MaxGas:   int64(txGas),
		},
		BlockGossip: BlockGossip{
			BlockPartSizeBytes: partSize,
		},
	}
}

func TestConsensusParamsHash(t *testing.T) {
	params := []ConsensusParams{
		makeParams(6, 2, 3, 4, 5),
		makeParams(1, 6, 3, 4, 5),
		makeParams(1, 2, 6, 4, 5),
		makeParams(1, 2, 3, 6, 5),
		makeParams(1, 2, 3, 4, 6),
	}

	hashes := make([][]byte, len(params))
	for i := range params {
		hashes[i] = params[i].Hash()
	}

	// make sure there are no duplicates...
	// sort, then check in order for matches
	sort.Slice(hashes, func(i, j int) bool {
		return bytes.Compare(hashes[i], hashes[j]) < 0
	})
	for i := 0; i < len(hashes)-1; i++ {
		assert.NotEqual(t, hashes[i], hashes[i+1])
	}
}

func TestConsensusParamsUpdate(t *testing.T) {
	testCases := []struct {
		params        ConsensusParams
		updates       *abci.ConsensusParams
		updatedParams ConsensusParams
	}{
		// empty updates
		{
			makeParams(1, 2, 3, 4, 5),
			&abci.ConsensusParams{},
			makeParams(1, 2, 3, 4, 5),
		},
		// negative BlockPartSizeBytes
		{
			makeParams(1, 2, 3, 4, 5),
			&abci.ConsensusParams{
				BlockSize: &abci.BlockSize{
					MaxBytes: -100,
					MaxGas:   -200,
				},
				TxSize: &abci.TxSize{
					MaxBytes: -400,
					MaxGas:   -500,
				},
				BlockGossip: &abci.BlockGossip{
					BlockPartSizeBytes: -600,
				},
			},
			makeParams(1, 2, 3, 4, 5),
		},
		// fine updates
		{
			makeParams(1, 2, 3, 4, 5),
			&abci.ConsensusParams{
				BlockSize: &abci.BlockSize{
					MaxBytes: 100,
					MaxGas:   200,
				},
				TxSize: &abci.TxSize{
					MaxBytes: 300,
					MaxGas:   400,
				},
				BlockGossip: &abci.BlockGossip{
					BlockPartSizeBytes: 500,
				},
			},
			makeParams(100, 200, 300, 400, 500),
		},
	}
	for _, tc := range testCases {
		assert.Equal(t, tc.updatedParams, tc.params.Update(tc.updates))
	}
}
