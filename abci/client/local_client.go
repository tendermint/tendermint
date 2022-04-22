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
	res := app.Application.Info(ctx, req)
	return &res, nil
}

func (app *localClient) CheckTx(ctx context.Context, req types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	res := app.Application.CheckTx(ctx, req)
	return &res, nil
}

func (app *localClient) Query(ctx context.Context, req types.RequestQuery) (*types.ResponseQuery, error) {
	res := app.Application.Query(ctx, req)
	return &res, nil
}

func (app *localClient) Commit(ctx context.Context) (*types.ResponseCommit, error) {
	res := app.Application.Commit(ctx)
	return &res, nil
}

func (app *localClient) InitChain(ctx context.Context, req types.RequestInitChain) (*types.ResponseInitChain, error) {
	res := app.Application.InitChain(ctx, req)
	return &res, nil
}

func (app *localClient) ListSnapshots(ctx context.Context, req types.RequestListSnapshots) (*types.ResponseListSnapshots, error) {
	res := app.Application.ListSnapshots(ctx, req)
	return &res, nil
}

func (app *localClient) OfferSnapshot(ctx context.Context, req types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
	res := app.Application.OfferSnapshot(ctx, req)
	return &res, nil
}

func (app *localClient) LoadSnapshotChunk(ctx context.Context, req types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {
	res := app.Application.LoadSnapshotChunk(ctx, req)
	return &res, nil
}

func (app *localClient) ApplySnapshotChunk(ctx context.Context, req types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {
	res := app.Application.ApplySnapshotChunk(ctx, req)
	return &res, nil
}

func (app *localClient) PrepareProposal(ctx context.Context, req types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {
	res := app.Application.PrepareProposal(ctx, req)
	return &res, nil
}

func (app *localClient) ProcessProposal(ctx context.Context, req types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {
	res := app.Application.ProcessProposal(ctx, req)
	return &res, nil
}

func (app *localClient) ExtendVote(ctx context.Context, req types.RequestExtendVote) (*types.ResponseExtendVote, error) {
	res := app.Application.ExtendVote(ctx, req)
	return &res, nil
}

func (app *localClient) VerifyVoteExtension(ctx context.Context, req types.RequestVerifyVoteExtension) (*types.ResponseVerifyVoteExtension, error) {
	res := app.Application.VerifyVoteExtension(ctx, req)
	return &res, nil
}

func (app *localClient) FinalizeBlock(ctx context.Context, req types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
	res := app.Application.FinalizeBlock(ctx, req)
	return &res, nil
}
