package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newConsensusParams(blockSize, partSize int) ConsensusParams {
	return ConsensusParams{
		BlockSizeParams:   BlockSizeParams{MaxBytes: blockSize},
		BlockGossipParams: BlockGossipParams{BlockPartSizeBytes: partSize},
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
