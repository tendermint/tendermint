package mempool

import "fmt"

// ErrPendingPoolIsFull means PendingPool can't handle that much load
type ErrPendingPoolIsFull struct {
	size    int
	maxSize int
}

func (e ErrPendingPoolIsFull) Error() string {
	return fmt.Sprintf(
		"Pending pool is full: current size %d, max size %d",
		e.size, e.maxSize)
}

// ErrPendingPoolAddressLimit means address sending too many txs in PendingPool
type ErrPendingPoolAddressLimit struct {
	address string
	size    int
	maxSize int
}

func (e ErrPendingPoolAddressLimit) Error() string {
	return fmt.Sprintf(
		"The address %s has too many txs in pending pool, current txs count %d, max count %d",
		e.address, e.size, e.maxSize,
	)
}

// ErrTxAlreadyInPendingPool means the tx already in PendingPool
type ErrTxAlreadyInPendingPool struct {
	txHash string
}

func (e ErrTxAlreadyInPendingPool) Error() string {
	return fmt.Sprintf(
		"Tx %s already exists in pending pool", e.txHash,
	)
}
