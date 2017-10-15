package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/types"
)

func TestConsensusParamsValidation(t *testing.T) {
	tests := [...]struct {
		params  *types.ConsensusParams
		wantErr string
	}{
		{&types.ConsensusParams{}, "BlockSizeParams.MaxBytes must be greater than 0"},
		{
			&types.ConsensusParams{BlockSizeParams: types.BlockSizeParams{MaxBytes: 10}},
			"BlockGossipParams.BlockPartSizeBytes must be greater than 0",
		},
		{
			&types.ConsensusParams{
				BlockSizeParams:   types.BlockSizeParams{MaxBytes: 10},
				BlockGossipParams: types.BlockGossipParams{BlockPartSizeBytes: -1},
			}, "BlockGossipParams.BlockPartSizeBytes must be greater than 0",
		},
		{
			&types.ConsensusParams{
				BlockSizeParams:   types.BlockSizeParams{MaxBytes: 10},
				BlockGossipParams: types.BlockGossipParams{BlockPartSizeBytes: 400},
			}, ""},
		{
			&types.ConsensusParams{
				BlockSizeParams:   types.BlockSizeParams{MaxBytes: 1024 * 1024 * 47},
				BlockGossipParams: types.BlockGossipParams{BlockPartSizeBytes: 400},
			}, "",
		},
		{
			&types.ConsensusParams{
				BlockSizeParams:   types.BlockSizeParams{MaxBytes: 1024 * 1024 * 100},
				BlockGossipParams: types.BlockGossipParams{BlockPartSizeBytes: 400},
			}, "",
		},
		{
			&types.ConsensusParams{
				BlockSizeParams:   types.BlockSizeParams{MaxBytes: 1024 * 1024 * 101},
				BlockGossipParams: types.BlockGossipParams{BlockPartSizeBytes: 400},
			}, "BlockSizeParams.MaxBytes is too big",
		},
		{
			&types.ConsensusParams{
				BlockSizeParams:   types.BlockSizeParams{MaxBytes: 1024 * 1024 * 1024},
				BlockGossipParams: types.BlockGossipParams{BlockPartSizeBytes: 400},
			}, "BlockSizeParams.MaxBytes is too big",
		},
	}

	for i, tt := range tests {
		err := tt.params.Validate()
		if tt.wantErr != "" {
			assert.Contains(t, err.Error(), tt.wantErr, "#%d: params: %#v", i, tt.params)
		} else {
			assert.Nil(t, err, "#%d: want nil error; Params: %#v", i, tt.params)
		}
	}
}
