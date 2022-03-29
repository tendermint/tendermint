package factory

import "github.com/tendermint/tendermint/types"

func MakeNTxs(height int64, n int) []types.Tx {
	txs := make([]types.Tx, n)
	for i := range txs {
		txs[i] = types.Tx([]byte{byte(height), byte(i/256), byte(i%256)})
	}
	return txs
}

func MakeTenTxs(height int64) []types.Tx {
	txs := make([]types.Tx, 10)
	for i := range txs {
		txs[i] = types.Tx([]byte{byte(height), byte(i)})
	}
	return txs
}
