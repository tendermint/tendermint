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

	conf, err := rpctest.CreateConfig(t.Name())
	conf.P2P.BootstrapPeers = "0123456789abcdef0123456789abcdef01234567@127.0.0.1" // placeholder
	require.NoError(t, err)

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
	_, conf := NodeSuite(t)

	res, err := rpctest.GetGRPCClient(conf).BroadcastTx(
		context.Background(),
		&coregrpc.RequestBroadcastTx{Tx: []byte("this is a tx")},
	)
	require.NoError(t, err)
	require.EqualValues(t, 0, res.CheckTx.Code)
	require.EqualValues(t, 0, res.DeliverTx.Code)
}
