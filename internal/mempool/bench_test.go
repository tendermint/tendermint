package mempool

import (
	"encoding/binary"
	"sync/atomic"
	"testing"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/proxy"
)

func BenchmarkReap(b *testing.B) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mempool, cleanup := newMempoolWithApp(cc)
	defer cleanup()
	mempool.config.Size = 100000

	size := 10000
	for i := 0; i < size; i++ {
		tx := make([]byte, 8)
		binary.BigEndian.PutUint64(tx, uint64(i))
		if err := mempool.CheckTx(tx, nil, TxInfo{}); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mempool.ReapMaxBytesMaxGas(100000000, 10000000)
	}
}

func BenchmarkCheckTx(b *testing.B) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mempool, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	mempool.config.Size = 1000000

	for i := 0; i < b.N; i++ {
		tx := make([]byte, 8)
		binary.BigEndian.PutUint64(tx, uint64(i))
		if err := mempool.CheckTx(tx, nil, TxInfo{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParallelCheckTx(b *testing.B) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mempool, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	mempool.config.Size = 100000000

	var txcnt uint64
	next := func() uint64 {
		return atomic.AddUint64(&txcnt, 1) - 1
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tx := make([]byte, 8)
			binary.BigEndian.PutUint64(tx, next())
			if err := mempool.CheckTx(tx, nil, TxInfo{}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCheckDuplicateTx(b *testing.B) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mempool, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	mempool.config.Size = 1000000

	for i := 0; i < b.N; i++ {
		tx := make([]byte, 8)
		binary.BigEndian.PutUint64(tx, uint64(i))
		if err := mempool.CheckTx(tx, nil, TxInfo{}); err != nil {
			b.Fatal(err)
		}

		if err := mempool.CheckTx(tx, nil, TxInfo{}); err == nil {
			b.Fatal("tx should be duplicate")
		}
	}
}

func BenchmarkCacheInsertTime(b *testing.B) {
	cache := newMapTxCache(b.N)
	txs := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		txs[i] = make([]byte, 8)
		binary.BigEndian.PutUint64(txs[i], uint64(i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Push(txs[i])
	}
}

// This benchmark is probably skewed, since we actually will be removing
// txs in parallel, which may cause some overhead due to mutex locking.
func BenchmarkCacheRemoveTime(b *testing.B) {
	cache := newMapTxCache(b.N)
	txs := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		txs[i] = make([]byte, 8)
		binary.BigEndian.PutUint64(txs[i], uint64(i))
		cache.Push(txs[i])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Remove(txs[i])
	}
}
