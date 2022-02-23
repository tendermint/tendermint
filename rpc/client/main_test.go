package client_test

import (
	"context"
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

func NodeSuite(ctx context.Context, t *testing.T, logger log.Logger) (service.Service, *config.Config) {
	t.Helper()

	ctx, cancel := context.WithCancel(ctx)

	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)

	// start an application. this could be the ephemeral
	// application *except* that application doesn't handle
	// evidence correctly, so we use this one.
	dir := t.TempDir()
	app := kvstore.NewPersistentKVStoreApplication(logger, dir)

	// start a tendermint node in the background to test against.
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
