package factory

import "github.com/tendermint/tendermint/types"

func MakeTenTxs(height int64) []types.Tx {
	txs := make([]types.Tx, 10)
	for i := range txs {
		txs[i] = types.Tx([]byte{byte(height), byte(i)})
	}
	return txs
}
