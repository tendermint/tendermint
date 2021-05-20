package v1

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/proxy"
)

func setup(t *testing.T) *TxMempool {
	t.Helper()

	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	cfg := config.ResetTestRoot(t.Name())

	appConnMem, err := cc.NewABCIClient()
	require.NoError(t, err)
	require.NoError(t, appConnMem.Start())

	t.Cleanup(func() {
		os.RemoveAll(cfg.RootDir)
	})

	return NewTxMempool(log.TestingLogger().With("test", t.Name()), cfg.Mempool, appConnMem, 0)
}
