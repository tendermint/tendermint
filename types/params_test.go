package types

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newConsensusParams(blockSize, partSize int) ConsensusParams {
	return ConsensusParams{
		BlockSize:   BlockSize{MaxBytes: blockSize},
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
	for _, testCase := range testCases {
		if testCase.valid {
			assert.NoError(t, testCase.params.Validate(), "expected no error for valid params")
		} else {
			assert.Error(t, testCase.params.Validate(), "expected error for non valid params")
		}
	}
}

func makeParams(blockBytes, blockTx, blockGas, txBytes,
	txGas, partSize int) ConsensusParams {

	return ConsensusParams{
		BlockSize: BlockSize{
			MaxBytes: blockBytes,
			MaxTxs:   blockTx,
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
		makeParams(1, 2, 3, 4, 5, 6),
		makeParams(7, 2, 3, 4, 5, 6),
		makeParams(1, 7, 3, 4, 5, 6),
		makeParams(1, 2, 7, 4, 5, 6),
		makeParams(1, 2, 3, 7, 5, 6),
		makeParams(1, 2, 3, 4, 7, 6),
		makeParams(1, 2, 3, 4, 5, 7),
		makeParams(6, 5, 4, 3, 2, 1),
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
