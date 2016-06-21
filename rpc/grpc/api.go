package core_grpc

import (
	core "github.com/tendermint/tendermint/rpc/core"

	context "golang.org/x/net/context"
)

type broadcastAPI struct {
}

func (bapi *broadcastAPI) BroadcastTx(ctx context.Context, req *RequestBroadcastTx) (*ResponseBroadcastTx, error) {
	res, err := core.BroadcastTxCommit(req.Tx)
	if res == nil {
		return nil, err
	}
	return &ResponseBroadcastTx{uint64(res.Code), res.Data, res.Log}, err
}
