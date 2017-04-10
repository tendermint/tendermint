package indexer

import (
	"errors"
	"github.com/tendermint/tendermint/types"
)

// Null acts as a /dev/null.
type Null struct{}

// Tx panics.
func (indexer *Null) Tx(hash []byte) (*types.TxResult, error) {
	return nil, errors.New(`Indexing is disabled (set 'tx_indexer = "kv"' in config)`)
}

// Batch returns nil.
func (indexer *Null) Batch(batch *Batch) error {
	return nil
}
