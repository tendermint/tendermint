//go:build gofuzz || go1.18

package tests

import (
	"testing"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	mempool "github.com/tendermint/tendermint/mempool"
	mempoolv1 "github.com/tendermint/tendermint/mempool/v1"
)

func FuzzMempool(f *testing.F) {
	app := kvstore.NewInMemoryApplication()
	logger := log.NewNopLogger()
	mtx := new(tmsync.Mutex)
	conn := abciclient.NewLocalClient(mtx, app)
	err := conn.Start()
	if err != nil {
		panic(err)
	}

	cfg := config.DefaultMempoolConfig()
	cfg.Broadcast = false

	mp := mempoolv1.NewTxMempool(logger, cfg, conn, 0)

	f.Fuzz(func(t *testing.T, data []byte) {
		_ = mp.CheckTx(data, nil, mempool.TxInfo{})
	})
}
