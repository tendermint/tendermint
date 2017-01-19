package indexer

import "github.com/tendermint/tendermint/types"

// Null acts as a /dev/null.
type Null struct{}

// Tx panics.
func (indexer *Null) Tx(hash string) (*types.TxResult, error) {
	panic("You are trying to get the transaction from a null indexer")
}

// Batch returns nil.
func (indexer *Null) Batch(batch *Batch) error {
	return nil
}
