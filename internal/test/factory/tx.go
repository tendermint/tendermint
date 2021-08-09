package factory

import "github.com/tendermint/tendermint/pkg/mempool"

// MakeTxs is a helper function to generate mock transactions by given the block height
// and the transaction numbers.
func MakeTxs(height int64, num int) (txs []mempool.Tx) {
	for i := 0; i < num; i++ {
		txs = append(txs, mempool.Tx([]byte{byte(height), byte(i)}))
	}
	return txs
}

func MakeTenTxs(height int64) (txs []mempool.Tx) {
	return MakeTxs(height, 10)
}
