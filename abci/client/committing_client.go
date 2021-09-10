package abcicli

import (
	"context"

	types "github.com/tendermint/tendermint/abci/types"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/libs/service"
)

var _ Client = (*committingClient)(nil)

// NOTE: use defer to unlock mutex because Application might panic (e.g., in
// case of malicious tx or query). It only makes sense for publicly exposed
// methods like CheckTx (/broadcast_tx_* RPC endpoint) or Query (/abci_query
// RPC endpoint), but defers are used everywhere for the sake of consistency.
type committingClient struct {
	service.BaseService

	// Only obtain a write lock when calling Application methods that are expected
	// to result in a state mutation.  This is currently:
	// SetOption
	// InitChain
	// Commit
	// ApplySnapshotChunk
	mtx *tmsync.RWMutex
	types.Application
	Callback
}

func NewCommittingClient(mtx *tmsync.RWMutex, app types.Application) Client {
	if mtx == nil {
		mtx = new(tmsync.RWMutex)
	}
	cli := &committingClient{
		mtx:         mtx,
		Application: app,
	}
	cli.BaseService = *service.NewBaseService(nil, "committingClient", cli)
	return cli
}

func (app *committingClient) SetResponseCallback(cb Callback) {
	// Write lock
	app.mtx.Lock()
	app.Callback = cb
	app.mtx.Unlock()
}

// TODO: change types.Application to include Error()?
func (app *committingClient) Error() error {
	return nil
}

func (app *committingClient) FlushAsync(ctx context.Context) (*ReqRes, error) {
	// Do nothing
	return newLocalReqRes(types.ToRequestFlush(), nil), nil
}

func (app *committingClient) EchoAsync(ctx context.Context, msg string) (*ReqRes, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	return app.callback(
		types.ToRequestEcho(msg),
		types.ToResponseEcho(msg),
	), nil
}

func (app *committingClient) InfoAsync(ctx context.Context, req types.RequestInfo) (*ReqRes, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.Info(req)
	return app.callback(
		types.ToRequestInfo(req),
		types.ToResponseInfo(res),
	), nil
}

func (app *committingClient) DeliverTxAsync(ctx context.Context, params types.RequestDeliverTx) (*ReqRes, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.DeliverTx(params)
	return app.callback(
		types.ToRequestDeliverTx(params),
		types.ToResponseDeliverTx(res),
	), nil
}

func (app *committingClient) CheckTxAsync(ctx context.Context, req types.RequestCheckTx) (*ReqRes, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.CheckTx(req)
	return app.callback(
		types.ToRequestCheckTx(req),
		types.ToResponseCheckTx(res),
	), nil
}

func (app *committingClient) QueryAsync(ctx context.Context, req types.RequestQuery) (*ReqRes, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.Query(req)
	return app.callback(
		types.ToRequestQuery(req),
		types.ToResponseQuery(res),
	), nil
}

func (app *committingClient) CommitAsync(ctx context.Context) (*ReqRes, error) {
	// Write lock
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.Commit()
	return app.callback(
		types.ToRequestCommit(),
		types.ToResponseCommit(res),
	), nil
}

func (app *committingClient) InitChainAsync(ctx context.Context, req types.RequestInitChain) (*ReqRes, error) {
	// Write lock
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.InitChain(req)
	return app.callback(
		types.ToRequestInitChain(req),
		types.ToResponseInitChain(res),
	), nil
}

func (app *committingClient) BeginBlockAsync(ctx context.Context, req types.RequestBeginBlock) (*ReqRes, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.BeginBlock(req)
	return app.callback(
		types.ToRequestBeginBlock(req),
		types.ToResponseBeginBlock(res),
	), nil
}

func (app *committingClient) EndBlockAsync(ctx context.Context, req types.RequestEndBlock) (*ReqRes, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.EndBlock(req)
	return app.callback(
		types.ToRequestEndBlock(req),
		types.ToResponseEndBlock(res),
	), nil
}

func (app *committingClient) ListSnapshotsAsync(ctx context.Context, req types.RequestListSnapshots) (*ReqRes, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.ListSnapshots(req)
	return app.callback(
		types.ToRequestListSnapshots(req),
		types.ToResponseListSnapshots(res),
	), nil
}

func (app *committingClient) OfferSnapshotAsync(ctx context.Context, req types.RequestOfferSnapshot) (*ReqRes, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.OfferSnapshot(req)
	return app.callback(
		types.ToRequestOfferSnapshot(req),
		types.ToResponseOfferSnapshot(res),
	), nil
}

func (app *committingClient) LoadSnapshotChunkAsync(
	ctx context.Context,
	req types.RequestLoadSnapshotChunk,
) (*ReqRes, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.LoadSnapshotChunk(req)
	return app.callback(
		types.ToRequestLoadSnapshotChunk(req),
		types.ToResponseLoadSnapshotChunk(res),
	), nil
}

func (app *committingClient) ApplySnapshotChunkAsync(
	ctx context.Context,
	req types.RequestApplySnapshotChunk,
) (*ReqRes, error) {
	// Write lock
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.ApplySnapshotChunk(req)
	return app.callback(
		types.ToRequestApplySnapshotChunk(req),
		types.ToResponseApplySnapshotChunk(res),
	), nil
}

//-------------------------------------------------------

func (app *committingClient) FlushSync(ctx context.Context) error {
	return nil
}

func (app *committingClient) EchoSync(ctx context.Context, msg string) (*types.ResponseEcho, error) {
	return &types.ResponseEcho{Message: msg}, nil
}

func (app *committingClient) InfoSync(ctx context.Context, req types.RequestInfo) (*types.ResponseInfo, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.Info(req)
	return &res, nil
}

func (app *committingClient) DeliverTxSync(
	ctx context.Context,
	req types.RequestDeliverTx,
) (*types.ResponseDeliverTx, error) {

	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.DeliverTx(req)
	return &res, nil
}

func (app *committingClient) CheckTxSync(
	ctx context.Context,
	req types.RequestCheckTx,
) (*types.ResponseCheckTx, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.CheckTx(req)
	return &res, nil
}

func (app *committingClient) QuerySync(
	ctx context.Context,
	req types.RequestQuery,
) (*types.ResponseQuery, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.Query(req)
	return &res, nil
}

func (app *committingClient) CommitSync(ctx context.Context) (*types.ResponseCommit, error) {
	// Write lock
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.Commit()
	return &res, nil
}

func (app *committingClient) InitChainSync(
	ctx context.Context,
	req types.RequestInitChain,
) (*types.ResponseInitChain, error) {

	// Write lock
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.InitChain(req)
	return &res, nil
}

func (app *committingClient) BeginBlockSync(
	ctx context.Context,
	req types.RequestBeginBlock,
) (*types.ResponseBeginBlock, error) {

	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.BeginBlock(req)
	return &res, nil
}

func (app *committingClient) EndBlockSync(
	ctx context.Context,
	req types.RequestEndBlock,
) (*types.ResponseEndBlock, error) {

	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.EndBlock(req)
	return &res, nil
}

func (app *committingClient) ListSnapshotsSync(
	ctx context.Context,
	req types.RequestListSnapshots,
) (*types.ResponseListSnapshots, error) {

	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.ListSnapshots(req)
	return &res, nil
}

func (app *committingClient) OfferSnapshotSync(
	ctx context.Context,
	req types.RequestOfferSnapshot,
) (*types.ResponseOfferSnapshot, error) {

	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.OfferSnapshot(req)
	return &res, nil
}

func (app *committingClient) LoadSnapshotChunkSync(
	ctx context.Context,
	req types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {

	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.LoadSnapshotChunk(req)
	return &res, nil
}

func (app *committingClient) ApplySnapshotChunkSync(
	ctx context.Context,
	req types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {

	// Write lock
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.ApplySnapshotChunk(req)
	return &res, nil
}

//-------------------------------------------------------

func (app *committingClient) callback(req *types.Request, res *types.Response) *ReqRes {
	app.Callback(req, res)
	return newLocalReqRes(req, res)
}
