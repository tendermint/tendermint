package indexer

import (
	"bytes"
	"fmt"

	db "github.com/tendermint/go-db"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
)

// KV is a simplest possible indexer, backed by Key-Value storage (levelDB).
// It could only index transaction by its identifier.
type KV struct {
	store db.DB
}

// NewKV returns new instance of KV indexer.
func NewKV(store db.DB) *KV {
	return &KV{store: store}
}

// Tx gets transaction from the KV storage and returns it or nil if the
// transaction is not found.
func (indexer *KV) Tx(hash string) (*types.TxResult, error) {
	if hash == "" {
		return nil, ErrorEmptyHash
	}

	rawBytes := indexer.store.Get([]byte(hash))
	if rawBytes == nil {
		return nil, nil
	}

	r := bytes.NewReader(rawBytes)
	var n int
	var err error
	txResult := wire.ReadBinary(&types.TxResult{}, r, 0, &n, &err).(*types.TxResult)
	if err != nil {
		return nil, fmt.Errorf("Error reading TxResult: %v", err)
	}

	return txResult, nil
}

// Batch writes a batch of transactions into the KV storage.
func (indexer *KV) Batch(b *Batch) error {
	storeBatch := indexer.store.NewBatch()
	for hash, result := range b.Ops {
		rawBytes := wire.BinaryBytes(&result)
		storeBatch.Set([]byte(hash), rawBytes)
	}
	storeBatch.Write()
	return nil
}
