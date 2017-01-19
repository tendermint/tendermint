package tx

import (
	txindexer "github.com/tendermint/tendermint/state/tx/indexer"
	"github.com/tendermint/tendermint/types"
)

// Indexer interface defines methods to index and search transactions.
type Indexer interface {

	// Batch analyzes, indexes or stores a batch of transactions.
	//
	// NOTE We do not specify Index method for analyzing a single transaction
	// here because it bears heavy perfomance loses. Almost all advanced indexers
	// support batching.
	Batch(b *txindexer.Batch) error

	// Tx returns specified transaction or nil if the transaction is not indexed
	// or stored.
	Tx(hash string) (*types.TxResult, error)
}
