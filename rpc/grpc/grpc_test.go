package coregrpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/service"
	coregrpc "github.com/tendermint/tendermint/rpc/grpc"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

func NodeSuite(t *testing.T) (service.Service, *config.Config) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	conf := rpctest.CreateConfig(t.Name())

	// start a tendermint node in the background to test against
	app := kvstore.NewApplication()

	node, closer, err := rpctest.StartTendermint(ctx, conf, app)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = closer(ctx)
		cancel()
	})
	return node, conf
}

func TestBroadcastTx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, conf := NodeSuite(t)

	client, closer := rpctest.GetGRPCClient(conf)
	defer func() { require.NoError(t, closer.Close()) }()
	res, err := client.BroadcastTx(ctx,
		&coregrpc.RequestBroadcastTx{Tx: []byte("this is a tx")},
	)
	require.NoError(t, err)
	require.EqualValues(t, 0, res.CheckTx.Code)
	require.EqualValues(t, 0, res.DeliverTx.Code)
}
