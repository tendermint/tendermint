package types

import (
	"bytes"
	"errors"
	"fmt"

	abci "github.com/tendermint/abci/types"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/merkle"
)

// Tx is an arbitrary byte array.
// NOTE: Tx has no types at this level, so when wire encoded it's just length-prefixed.
// Alternatively, it may make sense to add types here and let
// []byte be type 0x1 so we can have versioned txs if need be in the future.
type Tx []byte

// Hash computes the RIPEMD160 hash of the wire encoded transaction.
func (tx Tx) Hash() []byte {
	return wireHasher(tx).Hash()
}

// String returns the hex-encoded transaction as a string.
func (tx Tx) String() string {
	return fmt.Sprintf("Tx{%X}", []byte(tx))
}

// Txs is a slice of Tx.
type Txs []Tx

// Hash returns the simple Merkle root hash of the transactions.
func (txs Txs) Hash() []byte {
	// Recursive impl.
	// Copied from tmlibs/merkle to avoid allocations
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

// Index returns the index of this transaction in the list, or -1 if not found
func (txs Txs) Index(tx Tx) int {
	for i := range txs {
		if bytes.Equal(txs[i], tx) {
			return i
		}
	}
	return -1
}

// IndexByHash returns the index of this transaction hash in the list, or -1 if not found
func (txs Txs) IndexByHash(hash []byte) int {
	for i := range txs {
		if bytes.Equal(txs[i].Hash(), hash) {
			return i
		}
	}
	return -1
}

// Proof returns a simple merkle proof for this node.
//
// Panics if i < 0 or i >= len(txs)
//
// TODO: optimize this!
func (txs Txs) Proof(i int) TxProof {
	l := len(txs)
	hashers := make([]merkle.Hasher, l)
	for i := 0; i < l; i++ {
		hashers[i] = txs[i]
	}
	root, proofs := merkle.SimpleProofsFromHashers(hashers)

	return TxProof{
		Index:    i,
		Total:    l,
		RootHash: root,
		Data:     txs[i],
		Proof:    *proofs[i],
	}
}

// TxProof represents a Merkle proof of the presence of a transaction in the Merkle tree.
type TxProof struct {
	Index, Total int
	RootHash     cmn.HexBytes
	Data         Tx
	Proof        merkle.SimpleProof
}

// LeadHash returns the hash of the transaction this proof refers to.
func (tp TxProof) LeafHash() []byte {
	return tp.Data.Hash()
}

// Validate verifies the proof. It returns nil if the RootHash matches the dataHash argument,
// and if the proof is internally consistent. Otherwise, it returns a sensible error.
func (tp TxProof) Validate(dataHash []byte) error {
	if !bytes.Equal(dataHash, tp.RootHash) {
		return errors.New("Proof matches different data hash")
	}

	valid := tp.Proof.Verify(tp.Index, tp.Total, tp.LeafHash(), tp.RootHash)
	if !valid {
		return errors.New("Proof is not internally consistent")
	}
	return nil
}

// TxResult contains results of executing the transaction.
//
// One usage is indexing transaction results.
type TxResult struct {
	Height int64                  `json:"height"`
	Index  uint32                 `json:"index"`
	Tx     Tx                     `json:"tx"`
	Result abci.ResponseDeliverTx `json:"result"`
}
