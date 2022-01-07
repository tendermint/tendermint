package mock

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/tendermint/tendermint/crypto"
	dashtypes "github.com/tendermint/tendermint/dash/types"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/types"
)

// NewNodeAddress generates a string that is accepted as validator address.
// For given `n`, the address will always be the same.
func NewNodeAddress(n uint64) string {
	nodeID := make([]byte, 20)
	binary.LittleEndian.PutUint64(nodeID, n)
	if n == 0 {
		n = math.MaxUint16
	}
	return fmt.Sprintf("tcp://%x@127.0.0.1:%d", nodeID, uint16(n))
}

// NewValidator generates a validator with only fields needed for node selection filled.
// For the same `id`, mock validator will always have the same data (proTxHash, NodeID)
func NewValidator(id uint64) *types.Validator {
	address, err := dashtypes.ParseValidatorAddress(NewNodeAddress(id))
	if err != nil {
		panic(err)
	}
	return &types.Validator{
		ProTxHash:   NewProTxHash(id),
		NodeAddress: address,
	}
}

// NewValidators generates a slice containing `n` mock validators.
// Each element is generated using `mock.NewValidator()`.
func NewValidators(n uint64) []*types.Validator {
	vals := make([]*types.Validator, 0, n)
	for i := uint64(0); i < n; i++ {
		vals = append(vals, NewValidator(i))
	}
	return vals
}

// NewProTxHash generates a deterministic proTxHash.
// For the same `id`, generated data is always the same.
func NewProTxHash(id uint64) []byte {
	data := make([]byte, crypto.ProTxHashSize)
	binary.LittleEndian.PutUint64(data, id)
	return data
}

// NewQuorumHash generates a deterministic quorum hash.
// For the same `id`, generated data is always the same.
func NewQuorumHash(id uint64) []byte {
	data := make([]byte, crypto.QuorumHashSize)
	binary.LittleEndian.PutUint64(data, id)
	return data
}

// NewProTxHashes generates multiple deterministic proTxHash'es using mockProTxHash.
// Each argument will be passed to mockProTxHash.
func NewProTxHashes(ids ...uint64) []bytes.HexBytes {
	hashes := make([]bytes.HexBytes, 0, len(ids))
	for _, id := range ids {
		hashes = append(hashes, NewProTxHash(id))
	}
	return hashes
}

// ValidatorsProTxHashes returns slice of proTxHashes for provided list of validators
func ValidatorsProTxHashes(vals []*types.Validator) []bytes.HexBytes {
	hashes := make([]bytes.HexBytes, len(vals))
	for id, val := range vals {
		hashes[id] = val.ProTxHash
	}
	return hashes
}
