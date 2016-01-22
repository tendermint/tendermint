package proxy

import (
	tmspcli "github.com/tendermint/tmsp/client/golang"
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

func (app *localAppConn) EchoAsync(msg string) {
	app.mtx.Lock()
	msg2 := app.Application.Echo(msg)
	app.mtx.Unlock()
	app.Callback(
		tmsp.RequestEcho{msg},
		tmsp.ResponseEcho{msg2},
	)
}

func (app *localAppConn) FlushAsync() {
	// Do nothing
}

func (app *localAppConn) SetOptionAsync(key string, value string) {
	app.mtx.Lock()
	retCode := app.Application.SetOption(key, value)
	app.mtx.Unlock()
	app.Callback(
		tmsp.RequestSetOption{key, value},
		tmsp.ResponseSetOption{retCode},
	)
}

func (app *localAppConn) AppendTxAsync(tx []byte) {
	app.mtx.Lock()
	events, retCode := app.Application.AppendTx(tx)
	app.mtx.Unlock()
	app.Callback(
		tmsp.RequestAppendTx{tx},
		tmsp.ResponseAppendTx{retCode},
	)
	for _, event := range events {
		app.Callback(
			nil,
			tmsp.ResponseEvent{event},
		)
	}
}

func (app *localAppConn) CheckTxAsync(tx []byte) {
	app.mtx.Lock()
	retCode := app.Application.CheckTx(tx)
	app.mtx.Unlock()
	app.Callback(
		tmsp.RequestCheckTx{tx},
		tmsp.ResponseCheckTx{retCode},
	)
}

func (app *localAppConn) GetHashAsync() {
	app.mtx.Lock()
	hash, retCode := app.Application.GetHash()
	app.mtx.Unlock()
	app.Callback(
		tmsp.RequestGetHash{},
		tmsp.ResponseGetHash{retCode, hash},
	)
}

func (app *localAppConn) AddListenerAsync(key string) {
	app.mtx.Lock()
	retCode := app.Application.AddListener(key)
	app.mtx.Unlock()
	app.Callback(
		tmsp.RequestAddListener{key},
		tmsp.ResponseAddListener{retCode},
	)
}

func (app *localAppConn) RemListenerAsync(key string) {
	app.mtx.Lock()
	retCode := app.Application.RemListener(key)
	app.mtx.Unlock()
	app.Callback(
		tmsp.RequestRemListener{key},
		tmsp.ResponseRemListener{retCode},
	)
}

func (app *localAppConn) InfoSync() (info []string, err error) {
	app.mtx.Lock()
	info = app.Application.Info()
	app.mtx.Unlock()
	return info, nil
}

func (app *localAppConn) FlushSync() error {
	return nil
}

func (app *localAppConn) GetHashSync() (hash []byte, err error) {
	app.mtx.Lock()
	hash, retCode := app.Application.GetHash()
	app.mtx.Unlock()
	return hash, retCode.Error()
}
