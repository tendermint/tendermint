package rpctest

import (
	"testing"

	"golang.org/x/net/context"

	"github.com/tendermint/tendermint/rpc/grpc"
)

//-------------------------------------------

func TestBroadcastTx(t *testing.T) {
	res, err := clientGRPC.BroadcastTx(context.Background(), &core_grpc.RequestBroadcastTx{[]byte("this is a tx")})
	if err != nil {
		t.Fatal(err)
	}
	if res.CheckTx.Code != 0 {
		t.Fatalf("Non-zero check tx code: %d", res.CheckTx.Code)
	}
	if res.DeliverTx.Code != 0 {
		t.Fatalf("Non-zero append tx code: %d", res.DeliverTx.Code)
	}
}
