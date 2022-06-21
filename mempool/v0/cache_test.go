package v0

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

func TestCacheAfterUpdate(t *testing.T) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mp, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	// reAddIndices & txsInCache can have elements > numTxsToCreate
	// also assumes max index is 255 for convenience
	// txs in cache also checks order of elements
	tests := []struct {
		numTxsToCreate int
		updateIndices  []int
		reAddIndices   []int
		txsInCache     []int
	}{
		{1, []int{}, []int{1}, []int{1, 0}},    // adding new txs works
		{2, []int{1}, []int{}, []int{1, 0}},    // update doesn't remove tx from cache
		{2, []int{2}, []int{}, []int{2, 1, 0}}, // update adds new tx to cache
		{2, []int{1}, []int{1}, []int{1, 0}},   // re-adding after update doesn't make dupe
	}
	for tcIndex, tc := range tests {
		for i := 0; i < tc.numTxsToCreate; i++ {
			tx := types.Tx{byte(i)}
			err := mp.CheckTx(tx, nil, mempool.TxInfo{})
			require.NoError(t, err)
		}

		updateTxs := []types.Tx{}
		for _, v := range tc.updateIndices {
			tx := types.Tx{byte(v)}
			updateTxs = append(updateTxs, tx)
		}
		err := mp.Update(int64(tcIndex), updateTxs, abciResponses(len(updateTxs), abci.CodeTypeOK), nil, nil)
		require.NoError(t, err)

		for _, v := range tc.reAddIndices {
			tx := types.Tx{byte(v)}
			_ = mp.CheckTx(tx, nil, mempool.TxInfo{})
		}

		cache := mp.cache.(*mempool.LRUTxCache)
		node := cache.GetList().Front()
		counter := 0
		for node != nil {
			require.NotEqual(t, len(tc.txsInCache), counter,
				"cache larger than expected on testcase %d", tcIndex)

			nodeVal := node.Value.(types.TxKey)
			expectedBz := sha256.Sum256([]byte{byte(tc.txsInCache[len(tc.txsInCache)-counter-1])})
			// Reference for reading the errors:
			// >>> sha256('\x00').hexdigest()
			// '6e340b9cffb37a989ca544e6bb780a2c78901d3fb33738768511a30617afa01d'
			// >>> sha256('\x01').hexdigest()
			// '4bf5122f344554c53bde2ebb8cd2b7e3d1600ad631c385a5d7cce23c7785459a'
			// >>> sha256('\x02').hexdigest()
			// 'dbc1b4c900ffe48d575b5da5c638040125f65db0fe3e24494b76ea986457d986'

			require.EqualValues(t, expectedBz, nodeVal, "Equality failed on index %d, tc %d", counter, tcIndex)
			counter++
			node = node.Next()
		}
		require.Equal(t, len(tc.txsInCache), counter,
			"cache smaller than expected on testcase %d", tcIndex)
		mp.Flush()
	}
}
