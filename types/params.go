package types

import (
	"github.com/pkg/errors"
)

// ConsensusParams contains consensus critical parameters
// that determine the validity of blocks.
type ConsensusParams struct {
	MaxBlockSizeBytes  int `json:"max_block_size_bytes"`
	BlockPartSizeBytes int `json:"block_part_size_bytes"`
}

// DefaultConsensusParams returns a default ConsensusParams.
func DefaultConsensusParams() *ConsensusParams {
	return &ConsensusParams{
		MaxBlockSizeBytes:  22020096, // 21MB
		BlockPartSizeBytes: 65536,    // 64kB,
	}
}

// Validate validates the ConsensusParams to ensure all values
// are within their allowed limits, and returns an error if they are not.
func (params *ConsensusParams) Validate() error {
	if params.MaxBlockSizeBytes <= 0 {
		return errors.Errorf("MaxBlockSizeBytes must be greater than 0. Got %d", params.MaxBlockSizeBytes)
	}
	if params.BlockPartSizeBytes <= 0 {
		return errors.Errorf("BlockPartSizeBytes must be greater than 0. Got %d", params.BlockPartSizeBytes)
	}
	return nil
}
