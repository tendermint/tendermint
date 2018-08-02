package abcicli

import (
	"sync"

	types "github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
)

var _ Client = (*localClient)(nil)

type localClient struct {
	cmn.BaseService
	mtx *sync.Mutex
	types.Application
	Callback
}

func NewLocalClient(mtx *sync.Mutex, app types.Application) *localClient {
	if mtx == nil {
		mtx = new(sync.Mutex)
	}
	cli := &localClient{
		mtx:         mtx,
		Application: app,
	}
	cli.BaseService = *cmn.NewBaseService(nil, "localClient", cli)
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

func (app *localClient) FlushAsync() *ReqRes {
	// Do nothing
	return newLocalReqRes(types.ToRequestFlush(), nil)
}

func (app *localClient) EchoAsync(msg string) *ReqRes {
	return app.callback(
		types.ToRequestEcho(msg),
		types.ToResponseEcho(msg),
	)
}

func (app *localClient) InfoAsync(req types.RequestInfo) *ReqRes {
	app.mtx.Lock()
	res := app.Application.Info(req)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestInfo(req),
		types.ToResponseInfo(res),
	)
}

func (app *localClient) SetOptionAsync(req types.RequestSetOption) *ReqRes {
	app.mtx.Lock()
	res := app.Application.SetOption(req)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestSetOption(req),
		types.ToResponseSetOption(res),
	)
}

func (app *localClient) DeliverTxAsync(tx []byte) *ReqRes {
	app.mtx.Lock()
	res := app.Application.DeliverTx(tx)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestDeliverTx(tx),
		types.ToResponseDeliverTx(res),
	)
}

func (app *localClient) CheckTxAsync(tx []byte) *ReqRes {
	app.mtx.Lock()
	res := app.Application.CheckTx(tx)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestCheckTx(tx),
		types.ToResponseCheckTx(res),
	)
}

func (app *localClient) QueryAsync(req types.RequestQuery) *ReqRes {
	app.mtx.Lock()
	res := app.Application.Query(req)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestQuery(req),
		types.ToResponseQuery(res),
	)
}

func (app *localClient) CommitAsync() *ReqRes {
	app.mtx.Lock()
	res := app.Application.Commit()
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestCommit(),
		types.ToResponseCommit(res),
	)
}

func (app *localClient) InitChainAsync(req types.RequestInitChain) *ReqRes {
	app.mtx.Lock()
	res := app.Application.InitChain(req)
	reqRes := app.callback(
		types.ToRequestInitChain(req),
		types.ToResponseInitChain(res),
	)
	app.mtx.Unlock()
	return reqRes
}

func (app *localClient) BeginBlockAsync(req types.RequestBeginBlock) *ReqRes {
	app.mtx.Lock()
	res := app.Application.BeginBlock(req)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestBeginBlock(req),
		types.ToResponseBeginBlock(res),
	)
}

func (app *localClient) EndBlockAsync(req types.RequestEndBlock) *ReqRes {
	app.mtx.Lock()
	res := app.Application.EndBlock(req)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestEndBlock(req),
		types.ToResponseEndBlock(res),
	)
}

//-------------------------------------------------------

func (app *localClient) FlushSync() error {
	return nil
}

func (app *localClient) EchoSync(msg string) (*types.ResponseEcho, error) {
	return &types.ResponseEcho{Message: msg}, nil
}

func (app *localClient) InfoSync(req types.RequestInfo) (*types.ResponseInfo, error) {
	app.mtx.Lock()
	res := app.Application.Info(req)
	app.mtx.Unlock()
	return &res, nil
}

func (app *localClient) SetOptionSync(req types.RequestSetOption) (*types.ResponseSetOption, error) {
	app.mtx.Lock()
	res := app.Application.SetOption(req)
	app.mtx.Unlock()
	return &res, nil
}

func (app *localClient) DeliverTxSync(tx []byte) (*types.ResponseDeliverTx, error) {
	app.mtx.Lock()
	res := app.Application.DeliverTx(tx)
	app.mtx.Unlock()
	return &res, nil
}

func (app *localClient) CheckTxSync(tx []byte) (*types.ResponseCheckTx, error) {
	app.mtx.Lock()
	res := app.Application.CheckTx(tx)
	app.mtx.Unlock()
	return &res, nil
}

func (app *localClient) QuerySync(req types.RequestQuery) (*types.ResponseQuery, error) {
	app.mtx.Lock()
	res := app.Application.Query(req)
	app.mtx.Unlock()
	return &res, nil
}

func (app *localClient) CommitSync() (*types.ResponseCommit, error) {
	app.mtx.Lock()
	res := app.Application.Commit()
	app.mtx.Unlock()
	return &res, nil
}

func (app *localClient) InitChainSync(req types.RequestInitChain) (*types.ResponseInitChain, error) {
	app.mtx.Lock()
	res := app.Application.InitChain(req)
	app.mtx.Unlock()
	return &res, nil
}

func (app *localClient) BeginBlockSync(req types.RequestBeginBlock) (*types.ResponseBeginBlock, error) {
	app.mtx.Lock()
	res := app.Application.BeginBlock(req)
	app.mtx.Unlock()
	return &res, nil
}

func (app *localClient) EndBlockSync(req types.RequestEndBlock) (*types.ResponseEndBlock, error) {
	app.mtx.Lock()
	res := app.Application.EndBlock(req)
	app.mtx.Unlock()
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
