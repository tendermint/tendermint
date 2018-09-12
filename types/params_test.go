package types

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	abci "github.com/tendermint/tendermint/abci/types"
)

func newConsensusParams(txsBytes, evidenceAge int) ConsensusParams {
	return ConsensusParams{
		BlockSize:      BlockSize{MaxBytes: txsBytes},
		EvidenceParams: EvidenceParams{MaxAge: int64(evidenceAge)},
	}
}

func TestConsensusParamsValidation(t *testing.T) {
	testCases := []struct {
		params ConsensusParams
		valid  bool
	}{
		// test block size
		0: {newConsensusParams(1, 1), true},
		1: {newConsensusParams(0, 1), false},
		2: {newConsensusParams(47*1024*1024, 1), true},
		3: {newConsensusParams(10, 1), true},
		4: {newConsensusParams(100*1024*1024, 1), true},
		5: {newConsensusParams(101*1024*1024, 1), false},
		6: {newConsensusParams(1024*1024*1024, 1), false},
		7: {newConsensusParams(1024*1024*1024, -1), false},
		// test evidence age
		8: {newConsensusParams(1, 0), false},
		9: {newConsensusParams(1, -1), false},
	}
	for i, tc := range testCases {
		if tc.valid {
			assert.NoErrorf(t, tc.params.Validate(), "expected no error for valid params (#%d)", i)
		} else {
			assert.Errorf(t, tc.params.Validate(), "expected error for non valid params (#%d)", i)
		}
	}
}

func makeParams(txsBytes, blockGas, evidenceAge int) ConsensusParams {
	return ConsensusParams{
		BlockSize: BlockSize{
			MaxBytes: txsBytes,
			MaxGas:   int64(blockGas),
		},
		EvidenceParams: EvidenceParams{
			MaxAge: int64(evidenceAge),
		},
	}
}

func TestConsensusParamsHash(t *testing.T) {
	params := []ConsensusParams{
		makeParams(4, 2, 3),
		makeParams(1, 4, 3),
		makeParams(1, 2, 4),
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
			makeParams(1, 2, 3),
			&abci.ConsensusParams{},
			makeParams(1, 2, 3),
		},
		// fine updates
		{
			makeParams(1, 2, 3),
			&abci.ConsensusParams{
				BlockSize: &abci.BlockSize{
					MaxBytes: 100,
					MaxGas:   200,
				},
				EvidenceParams: &abci.EvidenceParams{
					MaxAge: 300,
				},
			},
			makeParams(100, 200, 300),
		},
	}
	for _, tc := range testCases {
		assert.Equal(t, tc.updatedParams, tc.params.Update(tc.updates))
	}
}
