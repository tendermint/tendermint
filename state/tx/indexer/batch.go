package indexer

import "github.com/tendermint/tendermint/types"

// A Batch groups together multiple Index operations you would like performed
// at the same time. The Batch structure is NOT thread-safe. You should only
// perform operations on a batch from a single thread at a time. Once batch
// execution has started, you may not modify it.
type Batch struct {
	Ops map[string]types.TxResult
}

// NewBatch creates a new Batch.
func NewBatch() *Batch {
	return &Batch{
		Ops: make(map[string]types.TxResult),
	}
}

// Index adds or updates entry for the given hash.
func (b *Batch) Index(hash string, result types.TxResult) error {
	if hash == "" {
		return ErrorEmptyHash
	}
	b.Ops[hash] = result
	return nil
}

// Size returns the total number of operations inside the batch.
func (b *Batch) Size() int {
	return len(b.Ops)
}
