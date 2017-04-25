package kv

import (
	"bytes"
	"fmt"

	db "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
)

// TxIndex is the simplest possible indexer, backed by Key-Value storage (levelDB).
// It could only index transaction by its identifier.
type TxIndex struct {
	store db.DB
}

// NewTxIndex returns new instance of TxIndex.
func NewTxIndex(store db.DB) *TxIndex {
	return &TxIndex{store: store}
}

// Get gets transaction from the TxIndex storage and returns it or nil if the
// transaction is not found.
func (txi *TxIndex) Get(hash []byte) (*types.TxResult, error) {
	if len(hash) == 0 {
		return nil, txindex.ErrorEmptyHash
	}

	rawBytes := txi.store.Get(hash)
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

// Batch writes a batch of transactions into the TxIndex storage.
func (txi *TxIndex) AddBatch(b *txindex.Batch) error {
	storeBatch := txi.store.NewBatch()
	for _, result := range b.Ops {
		rawBytes := wire.BinaryBytes(&result)
		storeBatch.Set(result.Tx.Hash(), rawBytes)
	}
	storeBatch.Write()
	return nil
}
