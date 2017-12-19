package types

import (
	"github.com/pkg/errors"

	"github.com/tendermint/tmlibs/merkle"
)

const (
	maxBlockSizeBytes = 104857600 // 100MB
)

// ConsensusParams contains consensus critical parameters
// that determine the validity of blocks.
type ConsensusParams struct {
	BlockSize   `json:"block_size_params"`
	TxSize      `json:"tx_size_params"`
	BlockGossip `json:"block_gossip_params"`
}

// BlockSize contain limits on the block size.
type BlockSize struct {
	MaxBytes int `json:"max_bytes"` // NOTE: must not be 0 nor greater than 100MB
	MaxTxs   int `json:"max_txs"`
	MaxGas   int `json:"max_gas"`
}

// TxSize contain limits on the tx size.
type TxSize struct {
	MaxBytes int `json:"max_bytes"`
	MaxGas   int `json:"max_gas"`
}

// BlockGossip determine consensus critical elements of how blocks are gossiped
type BlockGossip struct {
	BlockPartSizeBytes int `json:"block_part_size_bytes"` // NOTE: must not be 0
}

// DefaultConsensusParams returns a default ConsensusParams.
func DefaultConsensusParams() *ConsensusParams {
	return &ConsensusParams{
		DefaultBlockSize(),
		DefaultTxSize(),
		DefaultBlockGossip(),
	}
}

// DefaultBlockSize returns a default BlockSize.
func DefaultBlockSize() BlockSize {
	return BlockSize{
		MaxBytes: 22020096, // 21MB
		MaxTxs:   100000,
		MaxGas:   -1,
	}
}

// DefaultTxSize returns a default TxSize.
func DefaultTxSize() TxSize {
	return TxSize{
		MaxBytes: 10240, // 10kB
		MaxGas:   -1,
	}
}

// DefaultBlockGossip returns a default BlockGossip.
func DefaultBlockGossip() BlockGossip {
	return BlockGossip{
		BlockPartSizeBytes: 65536, // 64kB,
	}
}

// Validate validates the ConsensusParams to ensure all values
// are within their allowed limits, and returns an error if they are not.
func (params *ConsensusParams) Validate() error {
	// ensure some values are greater than 0
	if params.BlockSize.MaxBytes <= 0 {
		return errors.Errorf("BlockSize.MaxBytes must be greater than 0. Got %d", params.BlockSize.MaxBytes)
	}
	if params.BlockGossip.BlockPartSizeBytes <= 0 {
		return errors.Errorf("BlockGossip.BlockPartSizeBytes must be greater than 0. Got %d", params.BlockGossip.BlockPartSizeBytes)
	}

	// ensure blocks aren't too big
	if params.BlockSize.MaxBytes > maxBlockSizeBytes {
		return errors.Errorf("BlockSize.MaxBytes is too big. %d > %d",
			params.BlockSize.MaxBytes, maxBlockSizeBytes)
	}
	return nil
}

// Hash returns a merkle hash of the parameters to store
// in the block header
func (params *ConsensusParams) Hash() []byte {
	return merkle.SimpleHashFromMap(map[string]interface{}{
		"block_gossip_part_size_bytes": params.BlockGossip.BlockPartSizeBytes,
		"block_size_max_bytes":         params.BlockSize.MaxBytes,
		"block_size_max_gas":           params.BlockSize.MaxGas,
		"block_size_max_txs":           params.BlockSize.MaxTxs,
		"tx_size_max_bytes":            params.TxSize.MaxBytes,
		"tx_size_max_gas":              params.TxSize.MaxGas,
	})
}
