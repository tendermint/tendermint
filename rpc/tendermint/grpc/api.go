package core_grpc

import (
	core "github.com/tendermint/tendermint/rpc/tendermint/core"

	context "golang.org/x/net/context"
)

type broadcastAPI struct {
}

func (bapi *broadcastAPI) BroadcastTx(ctx context.Context, req *RequestBroadcastTx) (*ResponseBroadcastTx, error) {
	res, err := core.BroadcastTxCommit(req.Tx)
	if err != nil {
		return nil, err
	}
	return &ResponseBroadcastTx{res.CheckTx, res.DeliverTx}, nil
}
