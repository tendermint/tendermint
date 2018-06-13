package txindex

import (
	"errors"

	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/types"
)

// TxIndexer interface defines methods to index and search transactions.
type TxIndexer interface {

	// AddBatch analyzes, indexes and stores a batch of transactions.
	AddBatch(b *Batch) error

	// Index analyzes, indexes and stores a single transaction.
	Index(result *types.TxResult) error

	// Get returns the transaction specified by hash or nil if the transaction is not indexed
	// or stored.
	Get(hash []byte) (*types.TxResult, error)

	// Search allows you to query for transactions.
	Search(q *query.Query) ([]*types.TxResult, error)
}

//----------------------------------------------------
// Txs are written as a batch

// Batch groups together multiple Index operations to be performed at the same time.
// NOTE: Batch is NOT thread-safe and must not be modified after starting its execution.
type Batch struct {
	Ops []*types.TxResult
}

// NewBatch creates a new Batch.
func NewBatch(n int64) *Batch {
	return &Batch{
		Ops: make([]*types.TxResult, n),
	}
}

// Add or update an entry for the given result.Index.
func (b *Batch) Add(result *types.TxResult) error {
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
