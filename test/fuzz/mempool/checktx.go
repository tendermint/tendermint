package mempool

import (
	"context"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/libs/log"
)

func getMp(ctx context.Context) (mempool.Mempool, error) {
	app := kvstore.NewApplication()
	cc := abciclient.NewLocalCreator(app)
	appConnMem, _ := cc(log.NewNopLogger())
	err := appConnMem.Start(ctx)
	if err != nil {
		return nil, err
	}

	cfg := config.DefaultMempoolConfig()
	cfg.Broadcast = false

	return mempool.NewTxMempool(
		log.NewNopLogger(),
		cfg,
		appConnMem,
		0,
	), nil
}

func Fuzz(ctx context.Context, data []byte) error {
	mp, err := getMp(ctx)
	if err != nil {
		return err
	}

	err = mp.CheckTx(ctx, data, nil, mempool.TxInfo{})
	if err != nil {
		return err
	}

	return nil
}
