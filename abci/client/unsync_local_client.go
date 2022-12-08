package abcicli

import (
	"sync"

	types "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/service"
)

type unsyncLocalClient struct {
	service.BaseService

	types.Application

	// This mutex is exclusively used to protect the callback.
	mtx sync.RWMutex
	Callback
}

var _ Client = (*unsyncLocalClient)(nil)

// NewUnsyncLocalClient creates an unsynchronized local client, which will be
// directly calling the methods of the given app.
//
// Unlike NewLocalClient, it does not hold a mutex around the application, so
// it is up to the application to manage its synchronization properly.
func NewUnsyncLocalClient(app types.Application) Client {
	cli := &unsyncLocalClient{
		Application: app,
	}
	cli.BaseService = *service.NewBaseService(nil, "unsyncLocalClient", cli)
	return cli
}

func (app *unsyncLocalClient) SetResponseCallback(cb Callback) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	app.Callback = cb
}

// TODO: change types.Application to include Error()?
func (app *unsyncLocalClient) Error() error {
	return nil
}

func (app *unsyncLocalClient) FlushAsync() *ReqRes {
	// Do nothing
	return newLocalReqRes(types.ToRequestFlush(), nil)
}

func (app *unsyncLocalClient) EchoAsync(msg string) *ReqRes {
	return app.callback(
		types.ToRequestEcho(msg),
		types.ToResponseEcho(msg),
	)
}

func (app *unsyncLocalClient) InfoAsync(req types.RequestInfo) *ReqRes {
	res := app.Application.Info(req)
	return app.callback(
		types.ToRequestInfo(req),
		types.ToResponseInfo(res),
	)
}

func (app *unsyncLocalClient) DeliverTxAsync(params types.RequestDeliverTx) *ReqRes {
	res := app.Application.DeliverTx(params)
	return app.callback(
		types.ToRequestDeliverTx(params),
		types.ToResponseDeliverTx(res),
	)
}

func (app *unsyncLocalClient) CheckTxAsync(req types.RequestCheckTx) *ReqRes {
	res := app.Application.CheckTx(req)
	return app.callback(
		types.ToRequestCheckTx(req),
		types.ToResponseCheckTx(res),
	)
}

func (app *unsyncLocalClient) QueryAsync(req types.RequestQuery) *ReqRes {
	res := app.Application.Query(req)
	return app.callback(
		types.ToRequestQuery(req),
		types.ToResponseQuery(res),
	)
}

func (app *unsyncLocalClient) CommitAsync() *ReqRes {
	res := app.Application.Commit()
	return app.callback(
		types.ToRequestCommit(),
		types.ToResponseCommit(res),
	)
}

func (app *unsyncLocalClient) InitChainAsync(req types.RequestInitChain) *ReqRes {
	res := app.Application.InitChain(req)
	return app.callback(
		types.ToRequestInitChain(req),
		types.ToResponseInitChain(res),
	)
}

func (app *unsyncLocalClient) BeginBlockAsync(req types.RequestBeginBlock) *ReqRes {
	res := app.Application.BeginBlock(req)
	return app.callback(
		types.ToRequestBeginBlock(req),
		types.ToResponseBeginBlock(res),
	)
}

func (app *unsyncLocalClient) EndBlockAsync(req types.RequestEndBlock) *ReqRes {
	res := app.Application.EndBlock(req)
	return app.callback(
		types.ToRequestEndBlock(req),
		types.ToResponseEndBlock(res),
	)
}

func (app *unsyncLocalClient) ListSnapshotsAsync(req types.RequestListSnapshots) *ReqRes {
	res := app.Application.ListSnapshots(req)
	return app.callback(
		types.ToRequestListSnapshots(req),
		types.ToResponseListSnapshots(res),
	)
}

func (app *unsyncLocalClient) OfferSnapshotAsync(req types.RequestOfferSnapshot) *ReqRes {
	res := app.Application.OfferSnapshot(req)
	return app.callback(
		types.ToRequestOfferSnapshot(req),
		types.ToResponseOfferSnapshot(res),
	)
}

func (app *unsyncLocalClient) LoadSnapshotChunkAsync(req types.RequestLoadSnapshotChunk) *ReqRes {
	res := app.Application.LoadSnapshotChunk(req)
	return app.callback(
		types.ToRequestLoadSnapshotChunk(req),
		types.ToResponseLoadSnapshotChunk(res),
	)
}

func (app *unsyncLocalClient) ApplySnapshotChunkAsync(req types.RequestApplySnapshotChunk) *ReqRes {
	res := app.Application.ApplySnapshotChunk(req)
	return app.callback(
		types.ToRequestApplySnapshotChunk(req),
		types.ToResponseApplySnapshotChunk(res),
	)
}

func (app *unsyncLocalClient) PrepareProposalAsync(req types.RequestPrepareProposal) *ReqRes {
	res := app.Application.PrepareProposal(req)
	return app.callback(
		types.ToRequestPrepareProposal(req),
		types.ToResponsePrepareProposal(res),
	)
}

func (app *unsyncLocalClient) ProcessProposalAsync(req types.RequestProcessProposal) *ReqRes {
	res := app.Application.ProcessProposal(req)
	return app.callback(
		types.ToRequestProcessProposal(req),
		types.ToResponseProcessProposal(res),
	)
}

//-------------------------------------------------------

func (app *unsyncLocalClient) FlushSync() error {
	return nil
}

func (app *unsyncLocalClient) EchoSync(msg string) (*types.ResponseEcho, error) {
	return &types.ResponseEcho{Message: msg}, nil
}

func (app *unsyncLocalClient) InfoSync(req types.RequestInfo) (*types.ResponseInfo, error) {
	res := app.Application.Info(req)
	return &res, nil
}

func (app *unsyncLocalClient) DeliverTxSync(req types.RequestDeliverTx) (*types.ResponseDeliverTx, error) {
	res := app.Application.DeliverTx(req)
	return &res, nil
}

func (app *unsyncLocalClient) CheckTxSync(req types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	res := app.Application.CheckTx(req)
	return &res, nil
}

func (app *unsyncLocalClient) QuerySync(req types.RequestQuery) (*types.ResponseQuery, error) {
	res := app.Application.Query(req)
	return &res, nil
}

func (app *unsyncLocalClient) CommitSync() (*types.ResponseCommit, error) {
	res := app.Application.Commit()
	return &res, nil
}

func (app *unsyncLocalClient) InitChainSync(req types.RequestInitChain) (*types.ResponseInitChain, error) {
	res := app.Application.InitChain(req)
	return &res, nil
}

func (app *unsyncLocalClient) BeginBlockSync(req types.RequestBeginBlock) (*types.ResponseBeginBlock, error) {
	res := app.Application.BeginBlock(req)
	return &res, nil
}

func (app *unsyncLocalClient) EndBlockSync(req types.RequestEndBlock) (*types.ResponseEndBlock, error) {
	res := app.Application.EndBlock(req)
	return &res, nil
}

func (app *unsyncLocalClient) ListSnapshotsSync(req types.RequestListSnapshots) (*types.ResponseListSnapshots, error) {
	res := app.Application.ListSnapshots(req)
	return &res, nil
}

func (app *unsyncLocalClient) OfferSnapshotSync(req types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
	res := app.Application.OfferSnapshot(req)
	return &res, nil
}

func (app *unsyncLocalClient) LoadSnapshotChunkSync(
	req types.RequestLoadSnapshotChunk,
) (*types.ResponseLoadSnapshotChunk, error) {
	res := app.Application.LoadSnapshotChunk(req)
	return &res, nil
}

func (app *unsyncLocalClient) ApplySnapshotChunkSync(
	req types.RequestApplySnapshotChunk,
) (*types.ResponseApplySnapshotChunk, error) {
	res := app.Application.ApplySnapshotChunk(req)
	return &res, nil
}

func (app *unsyncLocalClient) PrepareProposalSync(req types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {
	res := app.Application.PrepareProposal(req)
	return &res, nil
}

func (app *unsyncLocalClient) ProcessProposalSync(req types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {
	res := app.Application.ProcessProposal(req)
	return &res, nil
}

//-------------------------------------------------------

func (app *unsyncLocalClient) callback(req *types.Request, res *types.Response) *ReqRes {
	app.mtx.RLock()
	defer app.mtx.RUnlock()
	app.Callback(req, res)
	rr := newLocalReqRes(req, res)
	rr.callbackInvoked = true
	return rr
}
