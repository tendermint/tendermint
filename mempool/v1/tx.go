package v1

import (
	"time"

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

	// Timestamp is the time at which the node first received the transaction from
	// a peer. It is used as a second dimension is prioritizing transactions when
	// two transactions have the same priority.
	Timestamp time.Time
}
