package core_grpc_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/tendermint/abci/example/dummy"
	"github.com/tendermint/tendermint/rpc/grpc"
	"github.com/tendermint/tendermint/rpc/test"
)

func TestMain(m *testing.M) {
	// start a tendermint node in the background to test against
	app := dummy.NewDummyApplication()
	node := rpctest.StartTendermint(app)
	code := m.Run()

	// and shut down proper at the end
	node.Stop()
	node.Wait()
	os.Exit(code)
}

func TestBroadcastTx(t *testing.T) {
	require := require.New(t)
	res, err := rpctest.GetGRPCClient().BroadcastTx(context.Background(), &core_grpc.RequestBroadcastTx{[]byte("this is a tx")})
	require.Nil(err, "%+v", err)
	require.EqualValues(0, res.CheckTx.Code)
	require.EqualValues(0, res.DeliverTx.Code)
}
