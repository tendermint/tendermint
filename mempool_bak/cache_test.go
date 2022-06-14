package mempool

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCacheRemove(t *testing.T) {
	cache := NewLRUTxCache(100)
	numTxs := 10

	txs := make([][]byte, numTxs)
	for i := 0; i < numTxs; i++ {
		// probability of collision is 2**-256
		txBytes := make([]byte, 32)
		_, err := rand.Read(txBytes)
		require.NoError(t, err)

		txs[i] = txBytes
		cache.Push(txBytes)

		// make sure its added to both the linked list and the map
		require.Equal(t, i+1, len(cache.cacheMap))
		require.Equal(t, i+1, cache.list.Len())
	}

	for i := 0; i < numTxs; i++ {
		cache.Remove(txs[i])
		// make sure its removed from both the map and the linked list
		require.Equal(t, numTxs-(i+1), len(cache.cacheMap))
		require.Equal(t, numTxs-(i+1), cache.list.Len())
	}
}
