package config

import (
	"fmt"
)

type ConsensusParams struct {
	MaxBlockSizeBytes  int `json:"max_block_size_bytes"`
	BlockPartSizeBytes int `json:"block_part_size_bytes"`
}

func DefaultConsensusParams() *ConsensusParams {
	return &ConsensusParams{
		MaxBlockSizeBytes:  22020096, // 21MB
		BlockPartSizeBytes: 65536,    // 64kB,
	}
}

func (params *ConsensusParams) Validate() error {
	if params.MaxBlockSizeBytes <= 0 {
		return fmt.Errorf("MaxBlockSizeBytes must be greater than 0. Got %d", params.MaxBlockSizeBytes)
	}
	if params.BlockPartSizeBytes <= 0 {
		return fmt.Errorf("BlockPartSizeBytes must be greater than 0. Got %d", params.BlockPartSizeBytes)
	}
	return nil
}
