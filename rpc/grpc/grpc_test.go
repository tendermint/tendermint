package coregrpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/node"
	core_grpc "github.com/tendermint/tendermint/rpc/grpc"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

func NodeSuite(t *testing.T) *node.Node {
	t.Helper()

	// start a tendermint node in the background to test against
	app := kvstore.NewApplication()
	node := rpctest.StartTendermint(app)
	t.Cleanup(func() {
		rpctest.StopTendermint(node)
	})
	return node
}

func TestBroadcastTx(t *testing.T) {
	n := NodeSuite(t)

	res, err := rpctest.GetGRPCClient(n.Config()).BroadcastTx(
		context.Background(),
		&core_grpc.RequestBroadcastTx{Tx: []byte("this is a tx")},
	)
	require.NoError(t, err)
	require.EqualValues(t, 0, res.CheckTx.Code)
	require.EqualValues(t, 0, res.DeliverTx.Code)
}
