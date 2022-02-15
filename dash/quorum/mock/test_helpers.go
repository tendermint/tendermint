package mock

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/types"
)

// NewNodeAddress generates a string that is accepted as validator address.
// For given `seed`, the address will always be the same.
func NewNodeAddress(seed uint16) string {
	seed = checkSeed(seed)
	nodeID := NewNodeID(seed)
	return fmt.Sprintf("%s://%s@127.0.0.1:%d", p2p.TCPProtocol, nodeID, seed)
}

// NewNodeID generates new deterministic node ID.
// For a given `uniqueID`, the node ID is always the same.
func NewNodeID(seed uint16) string {
	seed = checkSeed(seed)
	nodeID := make([]byte, 20)
	binary.LittleEndian.PutUint64(nodeID, uint64(seed))
	return hex.EncodeToString(nodeID)
}

// NewValidator generates a validator with only fields needed for node selection filled.
// For the same `seed`, mock validator will always have the same data (proTxHash, NodeID)
func NewValidator(seed uint16) *types.Validator {
	seed = checkSeed(seed)
	address, err := types.ParseValidatorAddress(NewNodeAddress(seed))
	if err != nil {
		panic(err)
	}
	return &types.Validator{
		ProTxHash:   NewProTxHash(seed),
		NodeAddress: address,
	}
}

// NewValidators generates a slice containing `n` mock validators.
// Each element is generated using `mock.NewValidator()`.
func NewValidators(n uint16) []*types.Validator {
	vals := make([]*types.Validator, 0, n)
	for i := uint16(1); i <= n; i++ {
		vals = append(vals, NewValidator(i))
	}
	return vals
}

// NewProTxHash generates a deterministic proTxHash.
// For the same `seed`, generated data is always the same.
func NewProTxHash(seed uint16) []byte {
	seed = checkSeed(seed)
	data := make([]byte, crypto.ProTxHashSize)
	binary.LittleEndian.PutUint64(data, uint64(seed))
	return data
}

// NewQuorumHash generates a deterministic quorum hash.
// For the same `seed`, generated data is always the same.
func NewQuorumHash(seed uint16) []byte {
	seed = checkSeed(seed)
	data := make([]byte, crypto.QuorumHashSize)
	binary.LittleEndian.PutUint64(data, uint64(seed))
	return data
}

// NewProTxHashes generates multiple deterministic proTxHash'es using mockProTxHash.
// Each argument will be passed to mockProTxHash. Each proTxHash will use provided `seed`
func NewProTxHashes(seeds ...uint16) []bytes.HexBytes {
	hashes := make([]bytes.HexBytes, 0, len(seeds))
	for _, seed := range seeds {
		hashes = append(hashes, NewProTxHash(seed))
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

func checkSeed(n uint16) uint16 {
	if n == 0 || n == math.MaxUint16 {
		panic(fmt.Sprintf("unsupported `n`: %d, should be 0 < n < %d", n, math.MaxUint16))
	}
	return n
}
