package proxy

import (
	tmspcli "github.com/tendermint/tmsp/client"
	tmsp "github.com/tendermint/tmsp/types"
	"sync"
)

type localAppConn struct {
	mtx *sync.Mutex
	tmsp.Application
	tmspcli.Callback
}

func NewLocalAppConn(mtx *sync.Mutex, app tmsp.Application) *localAppConn {
	return &localAppConn{
		mtx:         mtx,
		Application: app,
	}
}

func (app *localAppConn) SetResponseCallback(cb tmspcli.Callback) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	app.Callback = cb
}

// TODO: change tmsp.Application to include Error()?
func (app *localAppConn) Error() error {
	return nil
}

func (app *localAppConn) EchoAsync(msg string) *tmspcli.ReqRes {
	return app.callback(
		tmsp.RequestEcho(msg),
		tmsp.ResponseEcho(msg),
	)
}

func (app *localAppConn) FlushAsync() *tmspcli.ReqRes {
	// Do nothing
	return NewReqRes(tmsp.RequestFlush(), nil)
}

func (app *localAppConn) SetOptionAsync(key string, value string) *tmspcli.ReqRes {
	app.mtx.Lock()
	log := app.Application.SetOption(key, value)
	app.mtx.Unlock()
	return app.callback(
		tmsp.RequestSetOption(key, value),
		tmsp.ResponseSetOption(log),
	)
}

func (app *localAppConn) AppendTxAsync(tx []byte) *tmspcli.ReqRes {
	app.mtx.Lock()
	res := app.Application.AppendTx(tx)
	app.mtx.Unlock()
	return app.callback(
		tmsp.RequestAppendTx(tx),
		tmsp.ResponseAppendTx(res.Code, res.Data, res.Log),
	)
}

func (app *localAppConn) CheckTxAsync(tx []byte) *tmspcli.ReqRes {
	app.mtx.Lock()
	res := app.Application.CheckTx(tx)
	app.mtx.Unlock()
	return app.callback(
		tmsp.RequestCheckTx(tx),
		tmsp.ResponseCheckTx(res.Code, res.Data, res.Log),
	)
}

func (app *localAppConn) CommitAsync() *tmspcli.ReqRes {
	app.mtx.Lock()
	res := app.Application.Commit()
	app.mtx.Unlock()
	return app.callback(
		tmsp.RequestCommit(),
		tmsp.ResponseCommit(res.Code, res.Data, res.Log),
	)
}

func (app *localAppConn) InfoSync() (info string, err error) {
	app.mtx.Lock()
	info = app.Application.Info()
	app.mtx.Unlock()
	return info, nil
}

func (app *localAppConn) FlushSync() error {
	return nil
}

func (app *localAppConn) CommitSync() (res tmsp.Result) {
	app.mtx.Lock()
	res = app.Application.Commit()
	app.mtx.Unlock()
	return res
}

func (app *localAppConn) InitChainSync(validators []*tmsp.Validator) (err error) {
	app.mtx.Lock()
	if bcApp, ok := app.Application.(tmsp.BlockchainAware); ok {
		bcApp.InitChain(validators)
	}
	app.mtx.Unlock()
	return nil
}

func (app *localAppConn) EndBlockSync(height uint64) (changedValidators []*tmsp.Validator, err error) {
	app.mtx.Lock()
	if bcApp, ok := app.Application.(tmsp.BlockchainAware); ok {
		changedValidators = bcApp.EndBlock(height)
	}
	app.mtx.Unlock()
	return changedValidators, nil
}

//-------------------------------------------------------

func (app *localAppConn) callback(req *tmsp.Request, res *tmsp.Response) *tmspcli.ReqRes {
	app.Callback(req, res)
	return NewReqRes(req, res)
}

func NewReqRes(req *tmsp.Request, res *tmsp.Response) *tmspcli.ReqRes {
	reqRes := tmspcli.NewReqRes(req)
	reqRes.Response = res
	reqRes.SetDone()
	return reqRes
}
