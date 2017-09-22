// Copyright 2015 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"bytes"
	"errors"
	"fmt"

	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/go-wire/data"
	"github.com/tendermint/tmlibs/merkle"
)

// Tx represents a transaction, which may contain arbitrary bytes.
type Tx []byte

// Hash returns the hash of the go-wire encoded Tx.
// Tx has no types at this level, so go-wire encoding only adds length-prefix.
// NOTE: It may make sense to add types here one day and let []byte be type 0x1
// so we can have versioned txs if need be in the future.
func (tx Tx) Hash() []byte {
	return merkle.SimpleHashFromBinary(tx)
}

// String returns a string representation of the Tx.
func (tx Tx) String() string {
	return fmt.Sprintf("Tx{%X}", []byte(tx))
}

// Txs is a slice of Tx.
type Txs []Tx

// Hash returns the simple Merkle root hash of the Txs.
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
	hashables := make([]merkle.Hashable, l)
	for i := 0; i < l; i++ {
		hashables[i] = txs[i]
	}
	root, proofs := merkle.SimpleProofsFromHashables(hashables)

	return TxProof{
		Index:    i,
		Total:    l,
		RootHash: root,
		Data:     txs[i],
		Proof:    *proofs[i],
	}
}

type TxProof struct {
	Index, Total int
	RootHash     data.Bytes
	Data         Tx
	Proof        merkle.SimpleProof
}

func (tp TxProof) LeafHash() []byte {
	return tp.Data.Hash()
}

// Validate returns nil if it matches the dataHash, and is internally consistent
// otherwise, returns a sensible error
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
	Height uint64                 `json:"height"`
	Index  uint32                 `json:"index"`
	Tx     Tx                     `json:"tx"`
	Result abci.ResponseDeliverTx `json:"result"`
}
