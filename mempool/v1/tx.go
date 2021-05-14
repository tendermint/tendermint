package v1

import (
	"github.com/tendermint/tendermint/types"
)

// WrappedTx defines a wrapper around a raw transaction with additional metadata
// that is used for indexing.
type WrappedTx struct {
	// Tx represents the raw binary transaction data.
	Tx types.Tx

	// Priority defines the transaction's priority as specified by the application
	// in the ResponseCheckTx response.
	Priority int64

	// Sender defines the transaction's sender as specified by the application in
	// the ResponseCheckTx response.
	Sender string

	// heapIndex defines the index of the item in the heap
	heapIndex int
}
