package types

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/tmhash"
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
	BlockSize BlockSizeParams `json:"block_size"`
	Evidence  EvidenceParams  `json:"evidence"`
	Validator ValidatorParams `json:"validator"`
}

// HashedParams is a subset of ConsensusParams.
// It is amino encoded and hashed into
// the Header.ConsensusHash.
type HashedParams struct {
	BlockMaxBytes int64
	BlockMaxGas   int64
}

// BlockSizeParams define limits on the block size.
type BlockSizeParams struct {
	MaxBytes int64 `json:"max_bytes"`
	MaxGas   int64 `json:"max_gas"`
}

// EvidenceParams determine how we handle evidence of malfeasance
type EvidenceParams struct {
	MaxAge int64 `json:"max_age"` // only accept new evidence more recent than this
}

// ValidatorParams restrict the public key types validators can use.
// NOTE: uses ABCI pubkey naming, not Amino names.
type ValidatorParams struct {
	PubKeyTypes []string `json:"pub_key_types"`
}

// DefaultConsensusParams returns a default ConsensusParams.
func DefaultConsensusParams() *ConsensusParams {
	return &ConsensusParams{
		DefaultBlockSizeParams(),
		DefaultEvidenceParams(),
		DefaultValidatorParams(),
	}
}

// DefaultBlockSizeParams returns a default BlockSizeParams.
func DefaultBlockSizeParams() BlockSizeParams {
	return BlockSizeParams{
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

// DefaultValidatorParams returns a default ValidatorParams, which allows
// only ed25519 pubkeys.
func DefaultValidatorParams() ValidatorParams {
	return ValidatorParams{[]string{ABCIPubKeyTypeEd25519}}
}

func (params *ValidatorParams) IsValidPubkeyType(pubkeyType string) bool {
	for i := 0; i < len(params.PubKeyTypes); i++ {
		if params.PubKeyTypes[i] == pubkeyType {
			return true
		}
	}
	return false
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

	if params.Evidence.MaxAge <= 0 {
		return cmn.NewError("EvidenceParams.MaxAge must be greater than 0. Got %d",
			params.Evidence.MaxAge)
	}

	if len(params.Validator.PubKeyTypes) == 0 {
		return cmn.NewError("len(Validator.PubKeyTypes) must be greater than 0")
	}

	// Check if keyType is a known ABCIPubKeyType
	for i := 0; i < len(params.Validator.PubKeyTypes); i++ {
		keyType := params.Validator.PubKeyTypes[i]
		if _, ok := ABCIPubKeyTypesToAminoNames[keyType]; !ok {
			return cmn.NewError("params.Validator.PubKeyTypes[%d], %s, is an unknown pubkey type",
				i, keyType)
		}
	}

	return nil
}

// Hash returns a hash of a subset of the parameters to store in the block header.
// Only the Block.MaxBytes and Block.MaxGas are included in the hash.
// This allows the ConsensusParams to evolve more without breaking the block
// protocol. No need for a Merkle tree here, just a small struct to hash.
func (params *ConsensusParams) Hash() []byte {
	hasher := tmhash.New()
	bz := cdcEncode(HashedParams{
		params.BlockSize.MaxBytes,
		params.BlockSize.MaxGas,
	})
	if bz == nil {
		panic("cannot fail to encode ConsensusParams")
	}
	hasher.Write(bz)
	return hasher.Sum(nil)
}

func (params *ConsensusParams) Equals(params2 *ConsensusParams) bool {
	return params.BlockSize == params2.BlockSize &&
		params.Evidence == params2.Evidence &&
		cmn.StringSliceEqual(params.Validator.PubKeyTypes, params2.Validator.PubKeyTypes)
}

// Update returns a copy of the params with updates from the non-zero fields of p2.
// NOTE: note: must not modify the original
func (params ConsensusParams) Update(params2 *abci.ConsensusParams) ConsensusParams {
	res := params // explicit copy

	if params2 == nil {
		return res
	}

	// we must defensively consider any structs may be nil
	if params2.BlockSize != nil {
		res.BlockSize.MaxBytes = params2.BlockSize.MaxBytes
		res.BlockSize.MaxGas = params2.BlockSize.MaxGas
	}
	if params2.Evidence != nil {
		res.Evidence.MaxAge = params2.Evidence.MaxAge
	}
	if params2.Validator != nil {
		// Copy params2.Validator.PubkeyTypes, and set result's value to the copy.
		// This avoids having to initialize the slice to 0 values, and then write to it again.
		res.Validator.PubKeyTypes = append([]string{}, params2.Validator.PubKeyTypes...)
	}
	return res
}
