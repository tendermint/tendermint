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
