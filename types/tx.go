package types

import (
	"github.com/tendermint/go-merkle"
)

type Tx []byte

// NOTE: this is the hash of the go-wire encoded Tx.
// Tx has no types at this level, so just length-prefixed.
// Maybe it should just be the hash of the bytes tho?
func (tx Tx) Hash() []byte {
	return merkle.SimpleHashFromBinary(tx)
}

type Txs []Tx

func (txs Txs) Hash() []byte {
	// Recursive impl.
	// Copied from go-merkle to avoid allocations
	switch len(txs) {
	case 0:
		return nil
	case 1:
		return txs[0].Hash()
	default:
		left := Txs(txs[:(len(txs)+1)/2]).Hash()
		right := Txs(txs[(len(txs)+1)/2:]).Hash()
		return merkle.SimpleHashFromTwoHashes(left, right)
	}
}
