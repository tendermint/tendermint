package abcicli

import (
	"sync"

	. "github.com/tendermint/go-common"
	types "github.com/tendermint/abci/types"
)

type localClient struct {
	BaseService
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
	cli.BaseService = *NewBaseService(log, "localClient", cli)
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

func (app *localClient) InfoAsync() *ReqRes {
	app.mtx.Lock()
	resInfo := app.Application.Info()
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestInfo(),
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
		types.ToResponseDeliverTx(res.Code, res.Data, res.Log),
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

func (app *localClient) QueryAsync(tx []byte) *ReqRes {
	app.mtx.Lock()
	res := app.Application.Query(tx)
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestQuery(tx),
		types.ToResponseQuery(res.Code, res.Data, res.Log),
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

func (app *localClient) InitChainAsync(validators []*types.Validator) *ReqRes {
	app.mtx.Lock()
	if bcApp, ok := app.Application.(types.BlockchainAware); ok {
		bcApp.InitChain(validators)
	}
	reqRes := app.callback(
		types.ToRequestInitChain(validators),
		types.ToResponseInitChain(),
	)
	app.mtx.Unlock()
	return reqRes
}

func (app *localClient) BeginBlockAsync(hash []byte, header *types.Header) *ReqRes {
	app.mtx.Lock()
	if bcApp, ok := app.Application.(types.BlockchainAware); ok {
		bcApp.BeginBlock(hash, header)
	}
	app.mtx.Unlock()
	return app.callback(
		types.ToRequestBeginBlock(hash, header),
		types.ToResponseBeginBlock(),
	)
}

func (app *localClient) EndBlockAsync(height uint64) *ReqRes {
	app.mtx.Lock()
	var resEndBlock types.ResponseEndBlock
	if bcApp, ok := app.Application.(types.BlockchainAware); ok {
		resEndBlock = bcApp.EndBlock(height)
	}
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

func (app *localClient) EchoSync(msg string) (res types.Result) {
	return types.OK.SetData([]byte(msg))
}

func (app *localClient) InfoSync() (resInfo types.ResponseInfo, err error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	resInfo = app.Application.Info()
	return resInfo, nil
}

func (app *localClient) SetOptionSync(key string, value string) (res types.Result) {
	app.mtx.Lock()
	log := app.Application.SetOption(key, value)
	app.mtx.Unlock()
	return types.OK.SetLog(log)
}

func (app *localClient) DeliverTxSync(tx []byte) (res types.Result) {
	app.mtx.Lock()
	res = app.Application.DeliverTx(tx)
	app.mtx.Unlock()
	return res
}

func (app *localClient) CheckTxSync(tx []byte) (res types.Result) {
	app.mtx.Lock()
	res = app.Application.CheckTx(tx)
	app.mtx.Unlock()
	return res
}

func (app *localClient) QuerySync(query []byte) (res types.Result) {
	app.mtx.Lock()
	res = app.Application.Query(query)
	app.mtx.Unlock()
	return res
}

func (app *localClient) CommitSync() (res types.Result) {
	app.mtx.Lock()
	res = app.Application.Commit()
	app.mtx.Unlock()
	return res
}

func (app *localClient) InitChainSync(validators []*types.Validator) (err error) {
	app.mtx.Lock()
	if bcApp, ok := app.Application.(types.BlockchainAware); ok {
		bcApp.InitChain(validators)
	}
	app.mtx.Unlock()
	return nil
}

func (app *localClient) BeginBlockSync(hash []byte, header *types.Header) (err error) {
	app.mtx.Lock()
	if bcApp, ok := app.Application.(types.BlockchainAware); ok {
		bcApp.BeginBlock(hash, header)
	}
	app.mtx.Unlock()
	return nil
}

func (app *localClient) EndBlockSync(height uint64) (resEndBlock types.ResponseEndBlock, err error) {
	app.mtx.Lock()
	if bcApp, ok := app.Application.(types.BlockchainAware); ok {
		resEndBlock = bcApp.EndBlock(height)
	}
	app.mtx.Unlock()
	return resEndBlock, nil
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
