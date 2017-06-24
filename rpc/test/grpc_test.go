package rpctest

import (
	"testing"

	"golang.org/x/net/context"

	"github.com/stretchr/testify/require"
	core_grpc "github.com/tendermint/tendermint/rpc/grpc"
)

func TestBroadcastTx(t *testing.T) {
	require := require.New(t)
	res, err := GetGRPCClient().BroadcastTx(context.Background(), &core_grpc.RequestBroadcastTx{[]byte("this is a tx")})
	require.Nil(err, "%+v", err)
	require.EqualValues(0, res.CheckTx.Code)
	require.EqualValues(0, res.DeliverTx.Code)
}
