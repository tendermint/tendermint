package proxy

import (
	tmsp "github.com/tendermint/tmsp/types"
)

type localAppContext struct {
	tmsp.AppContext
	Callback
}

func NewLocalAppContext(app tmsp.AppContext) *localAppContext {
	return &localAppContext{
		AppContext: app,
	}
}

func (app *localAppContext) SetResponseCallback(cb Callback) {
	app.Callback = cb
}

// TODO: change tmsp.AppContext to include Error()?
func (app *localAppContext) Error() error {
	return nil
}

func (app *localAppContext) EchoAsync(msg string) {
	msg2 := app.AppContext.Echo(msg)
	app.Callback(
		tmsp.RequestEcho{msg},
		tmsp.ResponseEcho{msg2},
	)
}

func (app *localAppContext) FlushAsync() {
	// Do nothing
}

func (app *localAppContext) SetOptionAsync(key string, value string) {
	retCode := app.AppContext.SetOption(key, value)
	app.Callback(
		tmsp.RequestSetOption{key, value},
		tmsp.ResponseSetOption{retCode},
	)
}

func (app *localAppContext) AppendTxAsync(tx []byte) {
	events, retCode := app.AppContext.AppendTx(tx)
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

func (app *localAppContext) GetHashAsync() {
	hash, retCode := app.AppContext.GetHash()
	app.Callback(
		tmsp.RequestGetHash{},
		tmsp.ResponseGetHash{retCode, hash},
	)
}

func (app *localAppContext) CommitAsync() {
	retCode := app.AppContext.Commit()
	app.Callback(
		tmsp.RequestCommit{},
		tmsp.ResponseCommit{retCode},
	)
}

func (app *localAppContext) RollbackAsync() {
	retCode := app.AppContext.Rollback()
	app.Callback(
		tmsp.RequestRollback{},
		tmsp.ResponseRollback{retCode},
	)
}

func (app *localAppContext) AddListenerAsync(key string) {
	retCode := app.AppContext.AddListener(key)
	app.Callback(
		tmsp.RequestAddListener{key},
		tmsp.ResponseAddListener{retCode},
	)
}

func (app *localAppContext) RemListenerAsync(key string) {
	retCode := app.AppContext.RemListener(key)
	app.Callback(
		tmsp.RequestRemListener{key},
		tmsp.ResponseRemListener{retCode},
	)
}

func (app *localAppContext) InfoSync() (info []string, err error) {
	info = app.AppContext.Info()
	return info, nil
}

func (app *localAppContext) FlushSync() error {
	return nil
}

func (app *localAppContext) GetHashSync() (hash []byte, err error) {
	hash, retCode := app.AppContext.GetHash()
	return hash, retCode.Error()
}

func (app *localAppContext) CommitSync() (err error) {
	retCode := app.AppContext.Commit()
	return retCode.Error()
}

func (app *localAppContext) RollbackSync() (err error) {
	retCode := app.AppContext.Rollback()
	return retCode.Error()
}
