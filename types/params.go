package types

import (
	"github.com/pkg/errors"

	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/tmlibs/merkle"
)

const (
	maxBlockSizeBytes = 104857600 // 100MB
)

// ConsensusParams contains consensus critical parameters
// that determine the validity of blocks.
type ConsensusParams struct {
	BlockSize      `json:"block_size_params"`
	TxSize         `json:"tx_size_params"`
	BlockGossip    `json:"block_gossip_params"`
	EvidenceParams `json:"evidence_params"`
}

// BlockSize contain limits on the block size.
type BlockSize struct {
	MaxBytes int   `json:"max_bytes"` // NOTE: must not be 0 nor greater than 100MB
	MaxTxs   int   `json:"max_txs"`
	MaxGas   int64 `json:"max_gas"`
}

// TxSize contain limits on the tx size.
type TxSize struct {
	MaxBytes int   `json:"max_bytes"`
	MaxGas   int64 `json:"max_gas"`
}

// BlockGossip determine consensus critical elements of how blocks are gossiped
type BlockGossip struct {
	BlockPartSizeBytes int `json:"block_part_size_bytes"` // NOTE: must not be 0
}

// EvidenceParams determine how we handle evidence of malfeasance
type EvidenceParams struct {
	MaxAge int64 `json:"max_age"` // only accept new evidence more recent than this
}

// DefaultConsensusParams returns a default ConsensusParams.
func DefaultConsensusParams() *ConsensusParams {
	return &ConsensusParams{
		DefaultBlockSize(),
		DefaultTxSize(),
		DefaultBlockGossip(),
		DefaultEvidenceParams(),
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

// DefaultEvidence Params returns a default EvidenceParams.
func DefaultEvidenceParams() EvidenceParams {
	return EvidenceParams{
		MaxAge: 100000, // 27.8 hrs at 1block/s
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
	return merkle.SimpleHashFromMap(map[string]merkle.Hasher{
		"block_gossip_part_size_bytes": wireHasher(params.BlockGossip.BlockPartSizeBytes),
		"block_size_max_bytes":         wireHasher(params.BlockSize.MaxBytes),
		"block_size_max_gas":           wireHasher(params.BlockSize.MaxGas),
		"block_size_max_txs":           wireHasher(params.BlockSize.MaxTxs),
		"tx_size_max_bytes":            wireHasher(params.TxSize.MaxBytes),
		"tx_size_max_gas":              wireHasher(params.TxSize.MaxGas),
	})
}

// Update returns a copy of the params with updates from the non-zero fields of p2.
// NOTE: note: must not modify the original
func (params ConsensusParams) Update(params2 *abci.ConsensusParams) ConsensusParams {
	res := params // explicit copy

	if params2 == nil {
		return res
	}

	// we must defensively consider any structs may be nil
	// XXX: it's cast city over here. It's ok because we only do int32->int
	// but still, watch it champ.
	if params2.BlockSize != nil {
		if params2.BlockSize.MaxBytes > 0 {
			res.BlockSize.MaxBytes = int(params2.BlockSize.MaxBytes)
		}
		if params2.BlockSize.MaxTxs > 0 {
			res.BlockSize.MaxTxs = int(params2.BlockSize.MaxTxs)
		}
		if params2.BlockSize.MaxGas > 0 {
			res.BlockSize.MaxGas = params2.BlockSize.MaxGas
		}
	}
	if params2.TxSize != nil {
		if params2.TxSize.MaxBytes > 0 {
			res.TxSize.MaxBytes = int(params2.TxSize.MaxBytes)
		}
		if params2.TxSize.MaxGas > 0 {
			res.TxSize.MaxGas = params2.TxSize.MaxGas
		}
	}
	if params2.BlockGossip != nil {
		if params2.BlockGossip.BlockPartSizeBytes > 0 {
			res.BlockGossip.BlockPartSizeBytes = int(params2.BlockGossip.BlockPartSizeBytes)
		}
	}
	return res
}
