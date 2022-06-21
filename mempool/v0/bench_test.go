package v0

import (
	"encoding/binary"
	"sync/atomic"
	"testing"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
)

func BenchmarkReap(b *testing.B) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mp, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	mp.config.Size = 100000

	size := 10000
	for i := 0; i < size; i++ {
		tx := make([]byte, 8)
		binary.BigEndian.PutUint64(tx, uint64(i))
		if err := mp.CheckTx(tx, nil, mempool.TxInfo{}); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mp.ReapMaxBytesMaxGas(100000000, 10000000)
	}
}

func BenchmarkCheckTx(b *testing.B) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mp, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	mp.config.Size = 1000000

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		tx := make([]byte, 8)
		binary.BigEndian.PutUint64(tx, uint64(i))
		b.StartTimer()

		if err := mp.CheckTx(tx, nil, mempool.TxInfo{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParallelCheckTx(b *testing.B) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mp, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	mp.config.Size = 100000000

	var txcnt uint64
	next := func() uint64 {
		return atomic.AddUint64(&txcnt, 1) - 1
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tx := make([]byte, 8)
			binary.BigEndian.PutUint64(tx, next())
			if err := mp.CheckTx(tx, nil, mempool.TxInfo{}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCheckDuplicateTx(b *testing.B) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mp, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	mp.config.Size = 1000000

	for i := 0; i < b.N; i++ {
		tx := make([]byte, 8)
		binary.BigEndian.PutUint64(tx, uint64(i))
		if err := mp.CheckTx(tx, nil, mempool.TxInfo{}); err != nil {
			b.Fatal(err)
		}

		if err := mp.CheckTx(tx, nil, mempool.TxInfo{}); err == nil {
			b.Fatal("tx should be duplicate")
		}
	}
}
