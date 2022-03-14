package indexer

import (
	"context"
	"errors"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/pubsub/query"
	"github.com/tendermint/tendermint/types"
)

// TxIndexer interface defines methods to index and search transactions.
type TxIndexer interface {
	// Index analyzes, indexes and stores transactions. For indexing multiple
	// Transacions must guarantee the Index of the TxResult is in order.
	// See Batch struct.
	Index(results []*abci.TxResult) error

	// Get returns the transaction specified by hash or nil if the transaction is not indexed
	// or stored.
	Get(hash []byte) (*abci.TxResult, error)

	// Search allows you to query for transactions.
	Search(ctx context.Context, q *query.Query) ([]*abci.TxResult, error)
}

// BlockIndexer defines an interface contract for indexing block events.
type BlockIndexer interface {
	// Has returns true if the given height has been indexed. An error is returned
	// upon database query failure.
	Has(height int64) (bool, error)

	// Index indexes FinalizeBlock events for a given block by its height.
	Index(types.EventDataNewBlockHeader) error

	// Search performs a query for block heights that match a given FinalizeBlock
	// event search criteria.
	Search(ctx context.Context, q *query.Query) ([]int64, error)
}

// Batch groups together multiple Index operations to be performed at the same time.
// NOTE: Batch is NOT thread-safe and must not be modified after starting its execution.
type Batch struct {
	Ops     []*abci.TxResult
	Pending int64
}

// NewBatch creates a new Batch.
func NewBatch(n int64) *Batch {
	return &Batch{Ops: make([]*abci.TxResult, n), Pending: n}
}

// Add or update an entry for the given result.Index.
func (b *Batch) Add(result *abci.TxResult) error {
	if b.Ops[result.Index] == nil {
		b.Pending--
		b.Ops[result.Index] = result
	}
	return nil
}

// Size returns the total number of operations inside the batch.
func (b *Batch) Size() int { return len(b.Ops) }

// ErrorEmptyHash indicates empty hash
var ErrorEmptyHash = errors.New("transaction hash cannot be empty")
