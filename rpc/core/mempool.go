package core

import (
	"fmt"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

// Note: tx must be signed
func BroadcastTx(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	err := mempoolReactor.BroadcastTx(tx)
	if err != nil {
		return nil, fmt.Errorf("Error broadcasting transaction: %v", err)
	}
	return &ctypes.ResultBroadcastTx{}, nil
}

func ListUnconfirmedTxs() (*ctypes.ResultListUnconfirmedTxs, error) {
	txs, _, err := mempoolReactor.Mempool.Reap()
	return &ctypes.ResultListUnconfirmedTxs{len(txs), txs}, err
}
