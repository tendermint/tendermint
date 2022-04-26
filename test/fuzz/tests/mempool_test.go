//go:build gofuzz || go1.18

package tests

import (
	"context"
	"testing"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/libs/log"
)

var mp *mempool.TxMempool
var getMp func() mempool.Mempool

func init() {
	app := kvstore.NewApplication()
	logger := log.NewNopLogger()
	conn := abciclient.NewLocalClient(logger, app)
	err := conn.Start(context.TODO())
	if err != nil {
		panic(err)
	}

	cfg := config.DefaultMempoolConfig()
	cfg.Broadcast = false

	getMp = func() mempool.Mempool {
		if mp == nil {
			mp = mempool.NewTxMempool(logger, cfg, conn)
		}
		return mp
	}
}

func FuzzMempool(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		err := getMp().CheckTx(context.Background(), data, nil, mempool.TxInfo{})
		if err != nil {
			panic(err)
		}
	})
}
