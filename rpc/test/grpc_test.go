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
	if res.Code != 0 {
		t.Fatalf("Non-zero code: %d", res.Code)
	}
}
