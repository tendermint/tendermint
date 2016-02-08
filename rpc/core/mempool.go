package core

import (
	"fmt"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
	tmsp "github.com/tendermint/tmsp/types"
)

//-----------------------------------------------------------------------------

// NOTE: tx must be signed
func BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	err := mempoolReactor.BroadcastTx(tx, nil)
	if err != nil {
		return nil, fmt.Errorf("Error broadcasting transaction: %v", err)
	}
	return &ctypes.ResultBroadcastTx{}, nil
}

// Note: tx must be signed
func BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	resCh := make(chan *tmsp.Response, 1)
	err := mempoolReactor.BroadcastTx(tx, func(res *tmsp.Response) {
		resCh <- res
	})
	if err != nil {
		return nil, fmt.Errorf("Error broadcasting transaction: %v", err)
	}
	res := <-resCh
	return &ctypes.ResultBroadcastTx{
		Code: res.Code,
		Data: res.Data,
		Log:  res.Log,
	}, nil
}

func UnconfirmedTxs() (*ctypes.ResultUnconfirmedTxs, error) {
	txs, err := mempoolReactor.Mempool.Reap()
	return &ctypes.ResultUnconfirmedTxs{len(txs), txs}, err
}
