package abciclient

import (
	"context"
	"sync"

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

	mtx *sync.Mutex
	types.Application
	Callback
}

var _ Client = (*localClient)(nil)

// NewLocalClient creates a local client, which will be directly calling the
// methods of the given app.
//
// Both Async and Sync methods ignore the given context.Context parameter.
func NewLocalClient(logger log.Logger, mtx *sync.Mutex, app types.Application) Client {
	if mtx == nil {
		mtx = new(sync.Mutex)
	}
	cli := &localClient{
		mtx:         mtx,
		Application: app,
	}
	cli.BaseService = *service.NewBaseService(logger, "localClient", cli)
	return cli
}

func (*localClient) OnStart(context.Context) error { return nil }
func (*localClient) OnStop()                       {}

func (app *localClient) SetResponseCallback(cb Callback) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	app.Callback = cb
}

// TODO: change types.Application to include Error()?
func (app *localClient) Error() error {
	return nil
}

func (app *localClient) FlushAsync(ctx context.Context) (*ReqRes, error) {
	// Do nothing
	return newLocalReqRes(types.ToRequestFlush(), nil), nil
}

func (app *localClient) DeliverTxAsync(ctx context.Context, params types.RequestDeliverTx) (*ReqRes, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.DeliverTx(params)
	return app.callback(
		types.ToRequestDeliverTx(params),
		types.ToResponseDeliverTx(res),
	), nil
}

func (app *localClient) CheckTxAsync(ctx context.Context, req types.RequestCheckTx) (*ReqRes, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.CheckTx(req)
	return app.callback(
		types.ToRequestCheckTx(req),
		types.ToResponseCheckTx(res),
	), nil
}

//-------------------------------------------------------

func (app *localClient) Flush(ctx context.Context) error {
	return nil
}

func (app *localClient) Echo(ctx context.Context, msg string) (*types.ResponseEcho, error) {
	return &types.ResponseEcho{Message: msg}, nil
}

func (app *localClient) Info(ctx context.Context, req types.RequestInfo) (*types.ResponseInfo, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.Info(req)
	return &res, nil
}

func (app *localClient) DeliverTx(
	ctx context.Context,
	req types.RequestDeliverTx,
) (*types.ResponseDeliverTx, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.DeliverTx(req)
	return &res, nil
}

func (app *localClient) CheckTx(
	ctx context.Context,
	req types.RequestCheckTx,
) (*types.ResponseCheckTx, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.CheckTx(req)
	return &res, nil
}

func (app *localClient) Query(
	ctx context.Context,
	req types.RequestQuery,
) (*types.ResponseQuery, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.Query(req)
	return &res, nil
}

func (app *localClient) Commit(ctx context.Context) (*types.ResponseCommit, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.Commit()
	return &res, nil
}

func (app *localClient) InitChain(
	ctx context.Context,
	req types.RequestInitChain,
) (*types.ResponseInitChain, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.InitChain(req)
	return &res, nil
}

func (app *localClient) BeginBlock(
	ctx context.Context,
	req types.RequestBeginBlock,
) (*types.ResponseBeginBlock, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.BeginBlock(req)
	return &res, nil
}

func (app *localClient) EndBlock(
	ctx context.Context,
	req types.RequestEndBlock,
) (*types.ResponseEndBlock, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.EndBlock(req)
	return &res, nil
}

func (app *localClient) ListSnapshots(
	ctx context.Context,
	req types.RequestListSnapshots,
) (*types.ResponseListSnapshots, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.ListSnapshots(req)
	return &res, nil
}

func (app *localClient) OfferSnapshot(
	ctx context.Context,
	req types.RequestOfferSnapshot,
) (*types.ResponseOfferSnapshot, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.OfferSnapshot(req)
	return &res, nil
}

func (app *localClient) LoadSnapshotChunk(
	ctx context.Context,
	req types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.LoadSnapshotChunk(req)
	return &res, nil
}

func (app *localClient) ApplySnapshotChunk(
	ctx context.Context,
	req types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.ApplySnapshotChunk(req)
	return &res, nil
}

func (app *localClient) PrepareProposal(
	ctx context.Context,
	req types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.PrepareProposal(req)
	return &res, nil
}

func (app *localClient) ProcessProposal(
	ctx context.Context,
	req types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.ProcessProposal(req)
	return &res, nil
}

func (app *localClient) ExtendVote(
	ctx context.Context,
	req types.RequestExtendVote) (*types.ResponseExtendVote, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.ExtendVote(req)
	return &res, nil
}

func (app *localClient) VerifyVoteExtension(
	ctx context.Context,
	req types.RequestVerifyVoteExtension) (*types.ResponseVerifyVoteExtension, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.VerifyVoteExtension(req)
	return &res, nil
}

//-------------------------------------------------------

func (app *localClient) callback(req *types.Request, res *types.Response) *ReqRes {
	app.Callback(req, res)
	return newLocalReqRes(req, res)
}

func newLocalReqRes(req *types.Request, res *types.Response) *ReqRes {
	reqRes := NewReqRes(req)
	reqRes.Response = res
	reqRes.SetDone()
	return reqRes
}
