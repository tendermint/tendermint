package mempool

import (
	"errors"
	"fmt"
)

var (
	// ErrTxInCache is returned to the client if we saw tx earlier
	ErrTxInCache = errors.New("tx already exists in cache")
)

// ErrTxTooLarge means the tx is too big to be sent in a message to other peers
type ErrTxTooLarge struct {
	max    int
	actual int
}

func (e ErrTxTooLarge) Error() string {
	return fmt.Sprintf("Tx too large. Max size is %d, but got %d", e.max, e.actual)
}

// ErrMempoolIsFull means Tendermint & an application can't handle that much load
type ErrMempoolIsFull struct {
	numTxs int
	maxTxs int

	txsBytes    int64
	maxTxsBytes int64
}

func (e ErrMempoolIsFull) Error() string {
	return fmt.Sprintf(
		"mempool is full: number of txs %d (max: %d), total txs bytes %d (max: %d)",
		e.numTxs, e.maxTxs,
		e.txsBytes, e.maxTxsBytes)
}

// ErrPreCheck is returned when tx is too big
type ErrPreCheck struct {
	Reason error
}

func (e ErrPreCheck) Error() string {
	return e.Reason.Error()
}

// IsPreCheckError returns true if err is due to pre check failure.
func IsPreCheckError(err error) bool {
	_, ok := err.(ErrPreCheck)
	return ok
}
