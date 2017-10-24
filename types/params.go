package types

import (
	"github.com/pkg/errors"
)

const (
	maxBlockSizeBytes = 104857600 // 100MB
)

// ConsensusParams contains consensus critical parameters
// that determine the validity of blocks.
type ConsensusParams struct {
	BlockSizeParams   `json:"block_size_params"`
	TxSizeParams      `json:"tx_size_params"`
	BlockGossipParams `json:"block_gossip_params"`
}

// BlockSizeParams contain limits on the block size.
type BlockSizeParams struct {
	MaxBytes int `json:"max_bytes"` // NOTE: must not be 0 nor greater than 100MB
	MaxTxs   int `json:"max_txs"`
	MaxGas   int `json:"max_gas"`
}

// TxSizeParams contain limits on the tx size.
type TxSizeParams struct {
	MaxBytes int `json:"max_bytes"`
	MaxGas   int `json:"max_gas"`
}

// BlockGossipParams determine consensus critical elements of how blocks are gossiped
type BlockGossipParams struct {
	BlockPartSizeBytes int `json:"block_part_size_bytes"` // NOTE: must not be 0
}

// DefaultConsensusParams returns a default ConsensusParams.
func DefaultConsensusParams() *ConsensusParams {
	return &ConsensusParams{
		DefaultBlockSizeParams(),
		DefaultTxSizeParams(),
		DefaultBlockGossipParams(),
	}
}

// DefaultBlockSizeParams returns a default BlockSizeParams.
func DefaultBlockSizeParams() BlockSizeParams {
	return BlockSizeParams{
		MaxBytes: 22020096, // 21MB
		MaxTxs:   100000,
		MaxGas:   -1,
	}
}

// DefaultTxSizeParams returns a default TxSizeParams.
func DefaultTxSizeParams() TxSizeParams {
	return TxSizeParams{
		MaxBytes: 10240, // 10kB
		MaxGas:   -1,
	}
}

// DefaultBlockGossipParams returns a default BlockGossipParams.
func DefaultBlockGossipParams() BlockGossipParams {
	return BlockGossipParams{
		BlockPartSizeBytes: 65536, // 64kB,
	}
}

// Validate validates the ConsensusParams to ensure all values
// are within their allowed limits, and returns an error if they are not.
func (params *ConsensusParams) Validate() error {
	// ensure some values are greater than 0
	if params.BlockSizeParams.MaxBytes <= 0 {
		return errors.Errorf("BlockSizeParams.MaxBytes must be greater than 0. Got %d", params.BlockSizeParams.MaxBytes)
	}
	if params.BlockGossipParams.BlockPartSizeBytes <= 0 {
		return errors.Errorf("BlockGossipParams.BlockPartSizeBytes must be greater than 0. Got %d", params.BlockGossipParams.BlockPartSizeBytes)
	}

	// ensure blocks aren't too big
	if params.BlockSizeParams.MaxBytes > maxBlockSizeBytes {
		return errors.Errorf("BlockSizeParams.MaxBytes is too big. %d > %d",
			params.BlockSizeParams.MaxBytes, maxBlockSizeBytes)
	}
	return nil
}
