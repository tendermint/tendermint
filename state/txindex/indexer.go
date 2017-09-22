// Copyright 2017 Tendermint. All Rights Reserved.
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

package txindex

import (
	"errors"

	"github.com/tendermint/tendermint/types"
)

// Indexer interface defines methods to index and search transactions.
type TxIndexer interface {

	// Batch analyzes, indexes or stores a batch of transactions.
	//
	// NOTE We do not specify Index method for analyzing a single transaction
	// here because it bears heavy perfomance loses. Almost all advanced indexers
	// support batching.
	AddBatch(b *Batch) error

	// Tx returns specified transaction or nil if the transaction is not indexed
	// or stored.
	Get(hash []byte) (*types.TxResult, error)
}

//----------------------------------------------------
// Txs are written as a batch

// A Batch groups together multiple Index operations you would like performed
// at the same time. The Batch structure is NOT thread-safe. You should only
// perform operations on a batch from a single thread at a time. Once batch
// execution has started, you may not modify it.
type Batch struct {
	Ops []types.TxResult
}

// NewBatch creates a new Batch.
func NewBatch(n int) *Batch {
	return &Batch{
		Ops: make([]types.TxResult, n),
	}
}

// Index adds or updates entry for the given result.Index.
func (b *Batch) Add(result types.TxResult) error {
	b.Ops[result.Index] = result
	return nil
}

// Size returns the total number of operations inside the batch.
func (b *Batch) Size() int {
	return len(b.Ops)
}

//----------------------------------------------------
// Errors

// ErrorEmptyHash indicates empty hash
var ErrorEmptyHash = errors.New("Transaction hash cannot be empty")
