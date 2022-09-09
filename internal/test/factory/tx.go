package factory

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/types"
)

func MakeNTxs(height, n int64) types.Txs {
	txs := make([]types.Tx, n)
	for i := range txs {
		txs[i] = types.Tx([]byte{byte(height), byte(i / 256), byte(i % 256)})
	}
	return txs
}

func ExecTxResults(txs types.Txs) []*abci.ExecTxResult {
	resTxs := make([]*abci.ExecTxResult, len(txs))

	for i, tx := range txs {
		if len(tx) == 0 {
			return nil
		}
		if len(tx) > 0 {
			resTxs[i] = &abci.ExecTxResult{Code: abci.CodeTypeOK}
		} else {
			resTxs[i] = &abci.ExecTxResult{Code: abci.CodeTypeOK + 10} // error
		}
	}

	return resTxs
}
