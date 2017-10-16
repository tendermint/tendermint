package null

import (
	"errors"

	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
)

// TxIndex acts as a /dev/null.
type TxIndex struct{}

// Get with a hash panics.
func (txi *TxIndex) Get(hash []byte) (*types.TxResult, error) {
	return nil, errors.New(`Indexing is disabled (set 'tx_index = "kv"' in config)`)
}

// AddBatch returns nil.
func (txi *TxIndex) AddBatch(batch *txindex.Batch) error {
	return nil
}
