package mempool

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func BenchmarkTxMempool_CheckTx(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// setup the cache and the mempool number for hitting GetEvictableTxs during the
	// benchmark. 5000 is the current default mempool size in the TM config.
	txmp := setup(ctx, b, 10000)
	txmp.config.Size = 5000

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	const peerID = 1

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		b.StopTimer()
		prefix := make([]byte, 20)
		_, err := rng.Read(prefix)
		require.NoError(b, err)

		priority := int64(rng.Intn(9999-1000) + 1000)
		tx := []byte(fmt.Sprintf("sender-%d-%d=%X=%d", n, peerID, prefix, priority))
		txInfo := TxInfo{SenderID: uint16(peerID)}

		b.StartTimer()

		require.NoError(b, txmp.CheckTx(ctx, tx, nil, txInfo))
	}
}
