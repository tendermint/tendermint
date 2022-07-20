package mempool

import (
	"context"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/libs/log"
)

var (
	mp        mempool.MempoolABCI
	getMpABCI func() mempool.MempoolABCI
)

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

	getMpABCI = func() mempool.MempoolABCI {
		if mp == nil {
			pool := mempool.NewTxMempool(logger, cfg, conn)
			mp = mempool.NewABCI(cfg, conn, pool, nil)
		}
		return mp
	}
}

func Fuzz(data []byte) int {
	err := getMpABCI().CheckTx(context.Background(), data, nil, mempool.TxInfo{})
	if err != nil {
		return 0
	}

	return 1
}
