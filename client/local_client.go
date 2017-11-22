package abcicli

import (
	"sync"

	types "github.com/tendermint/abci/types"
	cmn "github.com/tendermint/tmlibs/common"
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
	resInfo := app.Application.Info(req)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestInfo(req),
		types.ToResponseInfo(resInfo),
	)
}

func (app *localClient) SetOptionAsync(key string, value string) *ReqRes {
	app.mtx.Lock()
	log := app.Application.SetOption(key, value)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestSetOption(key, value),
		types.ToResponseSetOption(log),
	)
}

func (app *localClient) DeliverTxAsync(tx []byte) *ReqRes {
	app.mtx.Lock()
	res := app.Application.DeliverTx(tx)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestDeliverTx(tx),
		types.ToResponseDeliverTx(res.Code, res.Data, res.Log, res.Tags),
	)
}

func (app *localClient) CheckTxAsync(tx []byte) *ReqRes {
	app.mtx.Lock()
	res := app.Application.CheckTx(tx)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestCheckTx(tx),
		types.ToResponseCheckTx(res.Code, res.Data, res.Log),
	)
}

func (app *localClient) QueryAsync(reqQuery types.RequestQuery) *ReqRes {
	app.mtx.Lock()
	resQuery := app.Application.Query(reqQuery)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestQuery(reqQuery),
		types.ToResponseQuery(resQuery),
	)
}

func (app *localClient) CommitAsync() *ReqRes {
	app.mtx.Lock()
	res := app.Application.Commit()
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestCommit(),
		types.ToResponseCommit(res.Code, res.Data, res.Log),
	)
}

func (app *localClient) InitChainAsync(params types.RequestInitChain) *ReqRes {
	app.mtx.Lock()
	app.Application.InitChain(params)
	reqRes := app.callback(
		types.ToRequestInitChain(params),
		types.ToResponseInitChain(),
	)
	app.mtx.Unlock()
	return reqRes
}

func (app *localClient) BeginBlockAsync(params types.RequestBeginBlock) *ReqRes {
	app.mtx.Lock()
	app.Application.BeginBlock(params)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestBeginBlock(params),
		types.ToResponseBeginBlock(),
	)
}

func (app *localClient) EndBlockAsync(height uint64) *ReqRes {
	app.mtx.Lock()
	resEndBlock := app.Application.EndBlock(height)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestEndBlock(height),
		types.ToResponseEndBlock(resEndBlock),
	)
}

//-------------------------------------------------------

func (app *localClient) FlushSync() error {
	return nil
}

func (app *localClient) EchoSync(msg string) (*types.ResponseEcho, error) {
	return &types.ResponseEcho{msg}, nil
}

func (app *localClient) InfoSync(req types.RequestInfo) (*types.ResponseInfo, error) {
	app.mtx.Lock()
	res := app.Application.Info(req)
	app.mtx.Unlock()
	return &res, nil
}

func (app *localClient) SetOptionSync(key string, value string) (log string, err error) {
	app.mtx.Lock()
	log = app.Application.SetOption(key, value)
	app.mtx.Unlock()
	return log, nil
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

func (app *localClient) InitChainSync(params types.RequestInitChain) error {
	app.mtx.Lock()
	app.Application.InitChain(params)
	app.mtx.Unlock()
	return nil
}

func (app *localClient) BeginBlockSync(params types.RequestBeginBlock) error {
	app.mtx.Lock()
	app.Application.BeginBlock(params)
	app.mtx.Unlock()
	return nil
}

func (app *localClient) EndBlockSync(height uint64) (*types.ResponseEndBlock, error) {
	app.mtx.Lock()
	res := app.Application.EndBlock(height)
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
