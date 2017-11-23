package checktx

import (
	"github.com/tendermint/abci/client"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

const addr = "0.0.0.0:8080"

func Fuzz(data []byte) int {
	c, err := abcicli.NewClient(addr, "socket", false)
	if err != nil {
		panic(err)
	}

	app := proxy.NewAppConnMempool(c)
	mcfg := config.DefaultMempoolConfig()
	mp := mempool.NewMempool(mcfg, app, 1)
	defer mp.CloseWAL()

	if err := mp.CheckTx(types.Tx(data), nil); err != nil {
		return 0
	}

	return 1
}
