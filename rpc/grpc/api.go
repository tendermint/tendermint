package core_grpc

import (
	"context"

	abci "github.com/tendermint/tendermint/abci/types"
	core "github.com/tendermint/tendermint/rpc/core"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
)

type broadcastAPI struct {
}

func (bapi *broadcastAPI) Ping(ctx context.Context, req *RequestPing) (*ResponsePing, error) {
	// kvstore so we can check if the server is up
	return &ResponsePing{}, nil
}

func (bapi *broadcastAPI) BroadcastTx(ctx context.Context, req *RequestBroadcastTx) (*ResponseBroadcastTx, error) {
	// XXX: empty context ... no remote addr!
	res, err := core.BroadcastTxCommit(&rpctypes.Context{}, req.Tx)
	if err != nil {
		return nil, err
	}
	return &ResponseBroadcastTx{

		CheckTx: &abci.ResponseCheckTx{
			Code: res.CheckTx.Code,
			Data: res.CheckTx.Data,
			Log:  res.CheckTx.Log,
		},
		DeliverTx: &abci.ResponseDeliverTx{
			Code: res.DeliverTx.Code,
			Data: res.DeliverTx.Data,
			Log:  res.DeliverTx.Log,
		},
	}, nil
}
