package core

import (
	"fmt"

	abci "github.com/tendermint/abci/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

func Tx(hash []byte, height, index int, prove bool) (*ctypes.ResultTx, error) {
	var deliverTx abci.ResponseDeliverTx
	if len(hash) > 0 {
		if height != 0 || index != 0 {
			return nil, fmt.Errorf("Invalid args. If hash is provided, height and index should not be")
		}

		r, err := txIndexer.Tx(hash)
		if err != nil {
			return nil, err
		}

		if r == nil {
			return &ctypes.ResultTx{}, fmt.Errorf("Tx (%X) not found", hash)
		}

		height = int(r.Height) // XXX
		index = int(r.Index)
		deliverTx = r.DeliverTx
	}

	if height <= 0 || height > blockStore.Height() {
		return nil, fmt.Errorf("Invalid height (%d) for blockStore at height %d", height, blockStore.Height())
	}

	block := blockStore.LoadBlock(height)

	if index < 0 || index >= len(block.Data.Txs) {
		return nil, fmt.Errorf("Index (%d) is out of range for block (%d) with %d txs", index, height, len(block.Data.Txs))
	}
	tx := block.Data.Txs[index]

	var proof types.TxProof
	if prove {
		proof = block.Data.Txs.Proof(index)
	}

	return &ctypes.ResultTx{
		Height:    height,
		Index:     index,
		DeliverTx: deliverTx,
		Tx:        tx,
		Proof:     proof,
	}, nil
}
