package selectpeers

import (
	"bytes"

	"github.com/tendermint/tendermint/crypto"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/types"
)

// sortableValidator is a `types.Validator` which can generate SortKey(), as specified in DIP-6
type sortableValidator struct {
	types.Validator
	quorumHash tmbytes.HexBytes
	sortKey    []byte
}

// newSortableValidator extends the validator with an option to generate DIP-6 compatible SortKey()
func newSortableValidator(validator types.Validator, quorumHash tmbytes.HexBytes) sortableValidator {
	sv := sortableValidator{
		Validator:  validator,
		quorumHash: make([]byte, crypto.QuorumHashSize),
	}
	copy(sv.quorumHash, quorumHash)
	sv.sortKey = calculateDIP6SortKey(sv.ProTxHash, sv.quorumHash)
	return sv
}

// equal returns info if this sortable validator is equal to the other one, based on the SortKey
func (v sortableValidator) equal(other sortableValidator) bool {
	return v.compare(other) == 0
}

// compare returns info if this sortable validator is smaller equal or bigger to the other one, based on the SortKey.
// It returns negative value when `v` is smaller than `other`, 0 when they are equal,
// and positive value when `v` is bigger than `other`.
func (v sortableValidator) compare(other sortableValidator) int {
	return bytes.Compare(v.sortKey, other.sortKey)
}

// calculateDIP6SortKey calculates new key for this SortableValidator, which is
// SHA256(proTxHash, quorumHash), as per DIP-6.
func calculateDIP6SortKey(proTxHash, quorumHash tmbytes.HexBytes) []byte {
	keyBytes := make([]byte, 0, len(proTxHash)+len(quorumHash))
	keyBytes = append(keyBytes, proTxHash...)
	keyBytes = append(keyBytes, quorumHash...)
	return crypto.Checksum(keyBytes)
}
