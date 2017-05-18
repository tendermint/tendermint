package core

import (
	"fmt"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/state/txindex/null"
	"github.com/tendermint/tendermint/types"
)

// Tx allow user to query the transaction results. `nil` could mean the
// transaction is in the mempool, invalidated, or was not send in the first
// place.
func Tx(hash []byte, prove bool) (*ctypes.ResultTx, error) {

	// if index is disabled, return error
	if _, ok := txIndexer.(*null.TxIndex); ok {
		return nil, fmt.Errorf("Transaction indexing is disabled.")
	}

	r, err := txIndexer.Get(hash)
	if err != nil {
		return nil, err
	}

	if r == nil {
		return nil, fmt.Errorf("Tx (%X) not found", hash)
	}

	height := int(r.Height) // XXX
	index := int(r.Index)

	var proof types.TxProof
	if prove {
		block := blockStore.LoadBlock(height)
		proof = block.Data.Txs.Proof(index)
	}

	return &ctypes.ResultTx{
		Height:   height,
		Index:    index,
		TxResult: r.Result.Result(),
		Tx:       r.Tx,
		Proof:    proof,
	}, nil
}
