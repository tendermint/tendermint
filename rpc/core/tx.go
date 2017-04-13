package core

import (
	"fmt"

	abci "github.com/tendermint/abci/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/state/tx/indexer"
	"github.com/tendermint/tendermint/types"
)

func Tx(hash []byte, height, index int, prove bool) (*ctypes.ResultTx, error) {

	// if index is disabled, we need a height
	_, indexerDisabled := txIndexer.(*indexer.Null)
	if indexerDisabled && height == 0 {
		return nil, fmt.Errorf("TxIndexer is disabled. Please specify a height to search for the tx by hash or index")
	}

	// hash and index must not be passed together
	if len(hash) > 0 && index != 0 {
		return nil, fmt.Errorf("Invalid args. Only one of hash and index may be provided")
	}

	// results
	var txResult abci.ResponseDeliverTx
	var tx types.Tx

	// if indexer is enabled and we have a hash,
	// fetch the tx result and set the height and index
	if !indexerDisabled && len(hash) > 0 {
		r, err := txIndexer.Tx(hash)
		if err != nil {
			return nil, err
		}

		if r == nil {
			return &ctypes.ResultTx{}, fmt.Errorf("Tx (%X) not found", hash)
		}

		height = int(r.Height) // XXX
		index = int(r.Index)
		txResult = r.DeliverTx
	}

	// height must be valid
	if height <= 0 || height > blockStore.Height() {
		return nil, fmt.Errorf("Invalid height (%d) for blockStore at height %d", height, blockStore.Height())
	}

	block := blockStore.LoadBlock(height)

	// index must be valid
	if index < 0 || index >= len(block.Data.Txs) {
		return nil, fmt.Errorf("Index (%d) is out of range for block (%d) with %d txs", index, height, len(block.Data.Txs))
	}

	// if indexer is disabled and we have a hash,
	// search for it in the list of txs
	if indexerDisabled && len(hash) > 0 {
		index = block.Data.Txs.IndexByHash(hash)
		if index < 0 {
			return nil, fmt.Errorf("Tx hash %X not found in block %d", hash, height)
		}

	}
	tx = block.Data.Txs[index]

	var proof types.TxProof
	if prove {
		proof = block.Data.Txs.Proof(index)
	}

	return &ctypes.ResultTx{
		Height:   height,
		Index:    index,
		TxResult: txResult,
		Tx:       tx,
		Proof:    proof,
	}, nil
}
