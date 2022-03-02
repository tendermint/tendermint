package abciclient

import (
	"context"

	types "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
)

// NOTE: use defer to unlock mutex because Application might panic (e.g., in
// case of malicious tx or query). It only makes sense for publicly exposed
// methods like CheckTx (/broadcast_tx_* RPC endpoint) or Query (/abci_query
// RPC endpoint), but defers are used everywhere for the sake of consistency.
type localClient struct {
	service.BaseService
	types.Application
}

var _ Client = (*localClient)(nil)

// NewLocalClient creates a local client, which will be directly calling the
// methods of the given app.
//
// The client methods ignore their context argument.
func NewLocalClient(logger log.Logger, app types.Application) Client {
	cli := &localClient{
		Application: app,
	}
	cli.BaseService = *service.NewBaseService(logger, "localClient", cli)
	return cli
}

func (*localClient) OnStart(context.Context) error { return nil }
func (*localClient) OnStop()                       {}
func (*localClient) Error() error                  { return nil }

//-------------------------------------------------------

func (*localClient) Flush(context.Context) error { return nil }

func (app *localClient) Echo(_ context.Context, msg string) (*types.ResponseEcho, error) {
	return &types.ResponseEcho{Message: msg}, nil
}

func (app *localClient) Info(ctx context.Context, req types.RequestInfo) (*types.ResponseInfo, error) {
	res := app.Application.Info(req)
	return &res, nil
}

func (app *localClient) CheckTx(_ context.Context, req types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	res := app.Application.CheckTx(req)
	return &res, nil
}

func (app *localClient) Query(_ context.Context, req types.RequestQuery) (*types.ResponseQuery, error) {
	res := app.Application.Query(req)
	return &res, nil
}

func (app *localClient) Commit(ctx context.Context) (*types.ResponseCommit, error) {
	res := app.Application.Commit()
	return &res, nil
}

func (app *localClient) InitChain(_ context.Context, req types.RequestInitChain) (*types.ResponseInitChain, error) {
	res := app.Application.InitChain(req)
	return &res, nil
}

func (app *localClient) ListSnapshots(_ context.Context, req types.RequestListSnapshots) (*types.ResponseListSnapshots, error) {
	res := app.Application.ListSnapshots(req)
	return &res, nil
}

func (app *localClient) OfferSnapshot(_ context.Context, req types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
	res := app.Application.OfferSnapshot(req)
	return &res, nil
}

func (app *localClient) LoadSnapshotChunk(_ context.Context, req types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {
	res := app.Application.LoadSnapshotChunk(req)
	return &res, nil
}

func (app *localClient) ApplySnapshotChunk(_ context.Context, req types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {
	res := app.Application.ApplySnapshotChunk(req)
	return &res, nil
}

func (app *localClient) PrepareProposal(_ context.Context, req types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {
	res := app.Application.PrepareProposal(req)
	return &res, nil
}

func (app *localClient) ProcessProposal(_ context.Context, req types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {
	res := app.Application.ProcessProposal(req)
	return &res, nil
}

func (app *localClient) ExtendVote(_ context.Context, req types.RequestExtendVote) (*types.ResponseExtendVote, error) {
	res := app.Application.ExtendVote(req)
	return &res, nil
}

func (app *localClient) VerifyVoteExtension(_ context.Context, req types.RequestVerifyVoteExtension) (*types.ResponseVerifyVoteExtension, error) {
	res := app.Application.VerifyVoteExtension(req)
	return &res, nil
}

func (app *localClient) FinalizeBlock(_ context.Context, req types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
	res := app.Application.FinalizeBlock(req)
	return &res, nil
}
