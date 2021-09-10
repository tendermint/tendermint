package abcicli

import (
	types "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/service"
	tmsync "github.com/tendermint/tendermint/libs/sync"
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

func (app *committingClient) FlushAsync() *ReqRes {
	// Do nothing
	return newLocalReqRes(types.ToRequestFlush(), nil)
}

func (app *committingClient) EchoAsync(msg string) *ReqRes {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	return app.callback(
		types.ToRequestEcho(msg),
		types.ToResponseEcho(msg),
	)
}

func (app *committingClient) InfoAsync(req types.RequestInfo) *ReqRes {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.Info(req)
	return app.callback(
		types.ToRequestInfo(req),
		types.ToResponseInfo(res),
	)
}

func (app *committingClient) SetOptionAsync(req types.RequestSetOption) *ReqRes {
	// Write lock
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.SetOption(req)
	return app.callback(
		types.ToRequestSetOption(req),
		types.ToResponseSetOption(res),
	)
}

func (app *committingClient) DeliverTxAsync(params types.RequestDeliverTx) *ReqRes {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.DeliverTx(params)
	return app.callback(
		types.ToRequestDeliverTx(params),
		types.ToResponseDeliverTx(res),
	)
}

func (app *committingClient) CheckTxAsync(req types.RequestCheckTx) *ReqRes {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.CheckTx(req)
	return app.callback(
		types.ToRequestCheckTx(req),
		types.ToResponseCheckTx(res),
	)
}

func (app *committingClient) QueryAsync(req types.RequestQuery) *ReqRes {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.Query(req)
	return app.callback(
		types.ToRequestQuery(req),
		types.ToResponseQuery(res),
	)
}

func (app *committingClient) CommitAsync() *ReqRes {
	// Write lock
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.Commit()
	return app.callback(
		types.ToRequestCommit(),
		types.ToResponseCommit(res),
	)
}

func (app *committingClient) InitChainAsync(req types.RequestInitChain) *ReqRes {
	// Write lock
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.InitChain(req)
	return app.callback(
		types.ToRequestInitChain(req),
		types.ToResponseInitChain(res),
	)
}

func (app *committingClient) BeginBlockAsync(req types.RequestBeginBlock) *ReqRes {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.BeginBlock(req)
	return app.callback(
		types.ToRequestBeginBlock(req),
		types.ToResponseBeginBlock(res),
	)
}

func (app *committingClient) EndBlockAsync(req types.RequestEndBlock) *ReqRes {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.EndBlock(req)
	return app.callback(
		types.ToRequestEndBlock(req),
		types.ToResponseEndBlock(res),
	)
}

func (app *committingClient) ListSnapshotsAsync(req types.RequestListSnapshots) *ReqRes {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.ListSnapshots(req)
	return app.callback(
		types.ToRequestListSnapshots(req),
		types.ToResponseListSnapshots(res),
	)
}

func (app *committingClient) OfferSnapshotAsync(req types.RequestOfferSnapshot) *ReqRes {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.OfferSnapshot(req)
	return app.callback(
		types.ToRequestOfferSnapshot(req),
		types.ToResponseOfferSnapshot(res),
	)
}

func (app *committingClient) LoadSnapshotChunkAsync(req types.RequestLoadSnapshotChunk) *ReqRes {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.LoadSnapshotChunk(req)
	return app.callback(
		types.ToRequestLoadSnapshotChunk(req),
		types.ToResponseLoadSnapshotChunk(res),
	)
}

func (app *committingClient) ApplySnapshotChunkAsync(req types.RequestApplySnapshotChunk) *ReqRes {
	// Write lock
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.ApplySnapshotChunk(req)
	return app.callback(
		types.ToRequestApplySnapshotChunk(req),
		types.ToResponseApplySnapshotChunk(res),
	)
}

//-------------------------------------------------------

func (app *committingClient) FlushSync() error {
	return nil
}

func (app *committingClient) EchoSync(msg string) (*types.ResponseEcho, error) {
	return &types.ResponseEcho{Message: msg}, nil
}

func (app *committingClient) InfoSync(req types.RequestInfo) (*types.ResponseInfo, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.Info(req)
	return &res, nil
}

func (app *committingClient) SetOptionSync(req types.RequestSetOption) (*types.ResponseSetOption, error) {
	// Write lock
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.SetOption(req)
	return &res, nil
}

func (app *committingClient) DeliverTxSync(req types.RequestDeliverTx) (*types.ResponseDeliverTx, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.DeliverTx(req)
	return &res, nil
}

func (app *committingClient) CheckTxSync(req types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.CheckTx(req)
	return &res, nil
}

func (app *committingClient) QuerySync(req types.RequestQuery) (*types.ResponseQuery, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.Query(req)
	return &res, nil
}

func (app *committingClient) CommitSync() (*types.ResponseCommit, error) {
	// Write lock
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.Commit()
	return &res, nil
}

func (app *committingClient) InitChainSync(req types.RequestInitChain) (*types.ResponseInitChain, error) {
	// Write lock
	app.mtx.Lock()
	defer app.mtx.Unlock()

	res := app.Application.InitChain(req)
	return &res, nil
}

func (app *committingClient) BeginBlockSync(req types.RequestBeginBlock) (*types.ResponseBeginBlock, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.BeginBlock(req)
	return &res, nil
}

func (app *committingClient) EndBlockSync(req types.RequestEndBlock) (*types.ResponseEndBlock, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.EndBlock(req)
	return &res, nil
}

func (app *committingClient) ListSnapshotsSync(req types.RequestListSnapshots) (*types.ResponseListSnapshots, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.ListSnapshots(req)
	return &res, nil
}

func (app *committingClient) OfferSnapshotSync(req types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.OfferSnapshot(req)
	return &res, nil
}

func (app *committingClient) LoadSnapshotChunkSync(
	req types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()

	res := app.Application.LoadSnapshotChunk(req)
	return &res, nil
}

func (app *committingClient) ApplySnapshotChunkSync(
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
