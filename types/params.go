package types

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	cmn "github.com/tendermint/tendermint/libs/common"
)

const (
	// MaxBlockSizeBytes is the maximum permitted size of the blocks.
	MaxBlockSizeBytes = 104857600 // 100MB

	// BlockPartSizeBytes is the size of one block part.
	BlockPartSizeBytes = 65536 // 64kB
)

// ConsensusParams contains consensus critical parameters that determine the
// validity of blocks.
type ConsensusParams struct {
	BlockSize      `json:"block_size_params"`
	EvidenceParams `json:"evidence_params"`
}

// BlockSize contain limits on the block size.
type BlockSize struct {
	MaxBytes int64 `json:"max_bytes"`
	MaxGas   int64 `json:"max_gas"`
}

// EvidenceParams determine how we handle evidence of malfeasance
type EvidenceParams struct {
	MaxAge int64 `json:"max_age"` // only accept new evidence more recent than this
}

// DefaultConsensusParams returns a default ConsensusParams.
func DefaultConsensusParams() *ConsensusParams {
	return &ConsensusParams{
		DefaultBlockSize(),
		DefaultEvidenceParams(),
	}
}

// DefaultBlockSize returns a default BlockSize.
func DefaultBlockSize() BlockSize {
	return BlockSize{
		MaxBytes: 22020096, // 21MB
		MaxGas:   -1,
	}
}

// DefaultEvidenceParams Params returns a default EvidenceParams.
func DefaultEvidenceParams() EvidenceParams {
	return EvidenceParams{
		MaxAge: 100000, // 27.8 hrs at 1block/s
	}
}

// Validate validates the ConsensusParams to ensure all values are within their
// allowed limits, and returns an error if they are not.
func (params *ConsensusParams) Validate() error {
	if params.BlockSize.MaxBytes <= 0 {
		return cmn.NewError("BlockSize.MaxBytes must be greater than 0. Got %d",
			params.BlockSize.MaxBytes)
	}
	if params.BlockSize.MaxBytes > MaxBlockSizeBytes {
		return cmn.NewError("BlockSize.MaxBytes is too big. %d > %d",
			params.BlockSize.MaxBytes, MaxBlockSizeBytes)
	}

	if params.BlockSize.MaxGas < -1 {
		return cmn.NewError("BlockSize.MaxGas must be greater or equal to -1. Got %d",
			params.BlockSize.MaxGas)
	}

	if params.EvidenceParams.MaxAge <= 0 {
		return cmn.NewError("EvidenceParams.MaxAge must be greater than 0. Got %d",
			params.EvidenceParams.MaxAge)
	}

	return nil
}

// Hash returns a merkle hash of the parameters to store in the block header
func (params *ConsensusParams) Hash() []byte {
	return merkle.SimpleHashFromMap(map[string]merkle.Hasher{
		"block_size_max_bytes":    aminoHasher(params.BlockSize.MaxBytes),
		"block_size_max_gas":      aminoHasher(params.BlockSize.MaxGas),
		"evidence_params_max_age": aminoHasher(params.EvidenceParams.MaxAge),
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
		res.BlockSize.MaxBytes = params2.BlockSize.MaxBytes
		res.BlockSize.MaxGas = params2.BlockSize.MaxGas
	}
	if params2.EvidenceParams != nil {
		res.EvidenceParams.MaxAge = params2.EvidenceParams.MaxAge
	}
	return res
}
