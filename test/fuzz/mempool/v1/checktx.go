package v1

import (
	"context"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/mempool"
	mempoolv1 "github.com/tendermint/tendermint/internal/mempool/v0"
)

var mp mempool.Mempool

func init() {
	app := kvstore.NewApplication()
	cc := abciclient.NewLocalCreator(app)
	appConnMem, _ := cc()
	err := appConnMem.Start()
	if err != nil {
		panic(err)
	}

	cfg := config.DefaultMempoolConfig()
	cfg.Broadcast = false

	mp = mempoolv1.NewCListMempool(cfg, appConnMem, 0)
}

func Fuzz(data []byte) int {
	err := mp.CheckTx(context.Background(), data, nil, mempool.TxInfo{})
	if err != nil {
		return 0
	}

	return 1
}
