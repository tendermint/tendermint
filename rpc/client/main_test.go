package client_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

func NodeSuite(t *testing.T, logger log.Logger) (service.Service, *config.Config) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	conf, err := rpctest.CreateConfig(t.Name())
	require.NoError(t, err)

	// start a tendermint node in the background to test against
	dir, err := os.MkdirTemp("/tmp", fmt.Sprint("rpc-client-test-", t.Name()))
	require.NoError(t, err)

	app := kvstore.NewPersistentKVStoreApplication(logger, dir)

	node, closer, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout)
	require.NoError(t, err)
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, closer(ctx))
		assert.NoError(t, app.Close())
		node.Wait()
		_ = os.RemoveAll(dir)
	})
	return node, conf
}
