package core_grpc

import (
	core "github.com/tendermint/tendermint/rpc/core"

	abci "github.com/tendermint/abci/types"

	context "golang.org/x/net/context"
)

type broadcastAPI struct {
}

func (bapi *broadcastAPI) BroadcastTx(ctx context.Context, req *RequestBroadcastTx) (*ResponseBroadcastTx, error) {
	res, err := core.BroadcastTxCommit(req.Tx)
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
