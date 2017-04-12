package core

import (
	"fmt"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

func Tx(hash []byte, prove bool) (*ctypes.ResultTx, error) {
	r, err := txIndexer.Tx(hash)
	if err != nil {
		return nil, err
	}

	if r == nil {
		return &ctypes.ResultTx{}, fmt.Errorf("Tx (%X) not found", hash)
	}

	block := blockStore.LoadBlock(int(r.Height))
	tx := block.Data.Txs[int(r.Index)]

	var proof types.TxProof
	if prove {
		proof = block.Data.Txs.Proof(int(r.Index))
	}

	return &ctypes.ResultTx{
		Height:    r.Height,
		Index:     r.Index,
		DeliverTx: r.DeliverTx,
		Tx:        tx,
		Proof:     proof,
	}, nil
}
