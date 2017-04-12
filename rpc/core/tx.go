package core

import (
	"fmt"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

func Tx(hash []byte) (*ctypes.ResultTx, error) {
	r, err := txIndexer.Tx(hash)
	if err != nil {
		return nil, err
	}

	if r == nil {
		return &ctypes.ResultTx{}, fmt.Errorf("Tx (%X) not found", hash)
	}

	block := blockStore.LoadBlock(int(r.Height))
	tx := block.Data.Txs[int(r.Index)]

	return &ctypes.ResultTx{
		Height:    r.Height,
		Index:     r.Index,
		DeliverTx: r.DeliverTx,
		Tx:        tx,
	}, nil
}
