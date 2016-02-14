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
	app.Callback(
		tmsp.RequestEcho(msg),
		tmsp.ResponseEcho(msg),
	)
	return nil // TODO maybe create a ReqRes
}

func (app *localAppConn) FlushAsync() *tmspcli.ReqRes {
	// Do nothing
	return nil // TODO maybe create a ReqRes
}

func (app *localAppConn) SetOptionAsync(key string, value string) *tmspcli.ReqRes {
	app.mtx.Lock()
	log := app.Application.SetOption(key, value)
	app.mtx.Unlock()
	app.Callback(
		tmsp.RequestSetOption(key, value),
		tmsp.ResponseSetOption(log),
	)
	return nil // TODO maybe create a ReqRes
}

func (app *localAppConn) AppendTxAsync(tx []byte) *tmspcli.ReqRes {
	app.mtx.Lock()
	code, result, log := app.Application.AppendTx(tx)
	app.mtx.Unlock()
	app.Callback(
		tmsp.RequestAppendTx(tx),
		tmsp.ResponseAppendTx(code, result, log),
	)
	return nil // TODO maybe create a ReqRes
}

func (app *localAppConn) CheckTxAsync(tx []byte) *tmspcli.ReqRes {
	app.mtx.Lock()
	code, result, log := app.Application.CheckTx(tx)
	app.mtx.Unlock()
	app.Callback(
		tmsp.RequestCheckTx(tx),
		tmsp.ResponseCheckTx(code, result, log),
	)
	return nil // TODO maybe create a ReqRes
}

func (app *localAppConn) CommitAsync() *tmspcli.ReqRes {
	app.mtx.Lock()
	hash, log := app.Application.Commit()
	app.mtx.Unlock()
	app.Callback(
		tmsp.RequestCommit(),
		tmsp.ResponseCommit(hash, log),
	)
	return nil // TODO maybe create a ReqRes
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

func (app *localAppConn) CommitSync() (hash []byte, log string, err error) {
	app.mtx.Lock()
	hash, log = app.Application.Commit()
	app.mtx.Unlock()
	return hash, log, nil
}
