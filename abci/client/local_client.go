package abcicli

import (
	"context"

	types "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/service"
	tmsync "github.com/tendermint/tendermint/libs/sync"
)

// NOTE: use defer to unlock mutex because Application might panic (e.g., in
// case of malicious tx or query). It only makes sense for publicly exposed
// methods like CheckTx (/broadcast_tx_* RPC endpoint) or Query (/abci_query
// RPC endpoint), but defers are used everywhere for the sake of consistency.
type localClient struct {
	service.BaseService

	mtx *tmsync.RWMutex
	types.Application
	Callback
}

var _ Client = (*localClient)(nil)

// NewLocalClient creates a local client, which will be directly calling the
// methods of the given app.
//
// Both Async and Sync methods ignore the given context.Context parameter.
func NewLocalClient(mtx *tmsync.RWMutex, app types.Application) Client {
	if mtx == nil {
		mtx = &tmsync.RWMutex{}
	}

	cli := &localClient{
		mtx:         mtx,
		Application: app,
	}

	cli.BaseService = *service.NewBaseService(nil, "localClient", cli)
	return cli
}

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

func (app *localClient) EchoAsync(ctx context.Context, msg string) (*ReqRes, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	return app.callback(
		types.ToRequestEcho(msg),
		types.ToResponseEcho(msg),
	), nil
}

func (app *localClient) InfoAsync(ctx context.Context, req types.RequestInfo) (*ReqRes, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.Info(req)
	return app.callback(
		types.ToRequestInfo(req),
		types.ToResponseInfo(res),
	), nil
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

func (app *localClient) QueryAsync(ctx context.Context, req types.RequestQuery) (*ReqRes, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.Query(req)
	return app.callback(
		types.ToRequestQuery(req),
		types.ToResponseQuery(res),
	), nil
}

func (app *localClient) CommitAsync(ctx context.Context) (*ReqRes, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.Commit()
	return app.callback(
		types.ToRequestCommit(),
		types.ToResponseCommit(res),
	), nil
}

func (app *localClient) InitChainAsync(ctx context.Context, req types.RequestInitChain) (*ReqRes, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.InitChain(req)
	return app.callback(
		types.ToRequestInitChain(req),
		types.ToResponseInitChain(res),
	), nil
}

func (app *localClient) BeginBlockAsync(ctx context.Context, req types.RequestBeginBlock) (*ReqRes, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.BeginBlock(req)
	return app.callback(
		types.ToRequestBeginBlock(req),
		types.ToResponseBeginBlock(res),
	), nil
}

func (app *localClient) EndBlockAsync(ctx context.Context, req types.RequestEndBlock) (*ReqRes, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.EndBlock(req)
	return app.callback(
		types.ToRequestEndBlock(req),
		types.ToResponseEndBlock(res),
	), nil
}

func (app *localClient) ListSnapshotsAsync(ctx context.Context, req types.RequestListSnapshots) (*ReqRes, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.ListSnapshots(req)
	return app.callback(
		types.ToRequestListSnapshots(req),
		types.ToResponseListSnapshots(res),
	), nil
}

func (app *localClient) OfferSnapshotAsync(ctx context.Context, req types.RequestOfferSnapshot) (*ReqRes, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.OfferSnapshot(req)
	return app.callback(
		types.ToRequestOfferSnapshot(req),
		types.ToResponseOfferSnapshot(res),
	), nil
}

func (app *localClient) LoadSnapshotChunkAsync(
	ctx context.Context,
	req types.RequestLoadSnapshotChunk,
) (*ReqRes, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.LoadSnapshotChunk(req)
	return app.callback(
		types.ToRequestLoadSnapshotChunk(req),
		types.ToResponseLoadSnapshotChunk(res),
	), nil
}

func (app *localClient) ApplySnapshotChunkAsync(
	ctx context.Context,
	req types.RequestApplySnapshotChunk,
) (*ReqRes, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.ApplySnapshotChunk(req)
	return app.callback(
		types.ToRequestApplySnapshotChunk(req),
		types.ToResponseApplySnapshotChunk(res),
	), nil
}

//-------------------------------------------------------

func (app *localClient) FlushSync(ctx context.Context) error {
	return nil
}

func (app *localClient) EchoSync(ctx context.Context, msg string) (*types.ResponseEcho, error) {
	return &types.ResponseEcho{Message: msg}, nil
}

func (app *localClient) InfoSync(ctx context.Context, req types.RequestInfo) (*types.ResponseInfo, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.Info(req)
	return &res, nil
}

func (app *localClient) DeliverTxSync(
	ctx context.Context,
	req types.RequestDeliverTx,
) (*types.ResponseDeliverTx, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.DeliverTx(req)
	return &res, nil
}

func (app *localClient) CheckTxSync(
	ctx context.Context,
	req types.RequestCheckTx,
) (*types.ResponseCheckTx, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.CheckTx(req)
	return &res, nil
}

func (app *localClient) QuerySync(
	ctx context.Context,
	req types.RequestQuery,
) (*types.ResponseQuery, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.Query(req)
	return &res, nil
}

func (app *localClient) CommitSync(ctx context.Context) (*types.ResponseCommit, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.Commit()
	return &res, nil
}

func (app *localClient) InitChainSync(
	ctx context.Context,
	req types.RequestInitChain,
) (*types.ResponseInitChain, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.InitChain(req)
	return &res, nil
}

func (app *localClient) BeginBlockSync(
	ctx context.Context,
	req types.RequestBeginBlock,
) (*types.ResponseBeginBlock, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.BeginBlock(req)
	return &res, nil
}

func (app *localClient) EndBlockSync(
	ctx context.Context,
	req types.RequestEndBlock,
) (*types.ResponseEndBlock, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.EndBlock(req)
	return &res, nil
}

func (app *localClient) ListSnapshotsSync(
	ctx context.Context,
	req types.RequestListSnapshots,
) (*types.ResponseListSnapshots, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.ListSnapshots(req)
	return &res, nil
}

func (app *localClient) OfferSnapshotSync(
	ctx context.Context,
	req types.RequestOfferSnapshot,
) (*types.ResponseOfferSnapshot, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.OfferSnapshot(req)
	return &res, nil
}

func (app *localClient) LoadSnapshotChunkSync(
	ctx context.Context,
	req types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.LoadSnapshotChunk(req)
	return &res, nil
}

func (app *localClient) ApplySnapshotChunkSync(
	ctx context.Context,
	req types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.ApplySnapshotChunk(req)
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
