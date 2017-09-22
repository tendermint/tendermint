package proxy

import (
	abcicli "github.com/tendermint/abci/client"
	"github.com/tendermint/abci/types"
)

//----------------------------------------------------------------------------------------
// Enforce which abci msgs can be sent on a connection at the type level

type AppConnConsensus interface {
	SetResponseCallback(abcicli.Callback)
	Error() error

	InitChainSync(types.RequestInitChain) (err error)

	BeginBlockSync(types.RequestBeginBlock) (err error)
	DeliverTxAsync(tx []byte) *abcicli.ReqRes
	EndBlockSync(height uint64) (types.ResponseEndBlock, error)
	CommitSync() (res types.Result)
}

type AppConnMempool interface {
	SetResponseCallback(abcicli.Callback)
	Error() error

	CheckTxAsync(tx []byte) *abcicli.ReqRes

	FlushAsync() *abcicli.ReqRes
	FlushSync() error
}

type AppConnQuery interface {
	Error() error

	EchoSync(string) (res types.Result)
	InfoSync(types.RequestInfo) (types.ResponseInfo, error)
	QuerySync(types.RequestQuery) (types.ResponseQuery, error)

	//	SetOptionSync(key string, value string) (res types.Result)
}

//-----------------------------------------------------------------------------------------
// Implements AppConnConsensus (subset of abcicli.Client)

type appConnConsensus struct {
	appConn abcicli.Client
}

func NewAppConnConsensus(appConn abcicli.Client) *appConnConsensus {
	return &appConnConsensus{
		appConn: appConn,
	}
}

func (app *appConnConsensus) SetResponseCallback(cb abcicli.Callback) {
	app.appConn.SetResponseCallback(cb)
}

func (app *appConnConsensus) Error() error {
	return app.appConn.Error()
}

func (app *appConnConsensus) InitChainSync(req types.RequestInitChain) (err error) {
	return app.appConn.InitChainSync(req)
}

func (app *appConnConsensus) BeginBlockSync(req types.RequestBeginBlock) (err error) {
	return app.appConn.BeginBlockSync(req)
}

func (app *appConnConsensus) DeliverTxAsync(tx []byte) *abcicli.ReqRes {
	return app.appConn.DeliverTxAsync(tx)
}

func (app *appConnConsensus) EndBlockSync(height uint64) (types.ResponseEndBlock, error) {
	return app.appConn.EndBlockSync(height)
}

func (app *appConnConsensus) CommitSync() (res types.Result) {
	return app.appConn.CommitSync()
}

//------------------------------------------------
// Implements AppConnMempool (subset of abcicli.Client)

type appConnMempool struct {
	appConn abcicli.Client
}

func NewAppConnMempool(appConn abcicli.Client) *appConnMempool {
	return &appConnMempool{
		appConn: appConn,
	}
}

func (app *appConnMempool) SetResponseCallback(cb abcicli.Callback) {
	app.appConn.SetResponseCallback(cb)
}

func (app *appConnMempool) Error() error {
	return app.appConn.Error()
}

func (app *appConnMempool) FlushAsync() *abcicli.ReqRes {
	return app.appConn.FlushAsync()
}

func (app *appConnMempool) FlushSync() error {
	return app.appConn.FlushSync()
}

func (app *appConnMempool) CheckTxAsync(tx []byte) *abcicli.ReqRes {
	return app.appConn.CheckTxAsync(tx)
}

//------------------------------------------------
// Implements AppConnQuery (subset of abcicli.Client)

type appConnQuery struct {
	appConn abcicli.Client
}

func NewAppConnQuery(appConn abcicli.Client) *appConnQuery {
	return &appConnQuery{
		appConn: appConn,
	}
}

func (app *appConnQuery) Error() error {
	return app.appConn.Error()
}

func (app *appConnQuery) EchoSync(msg string) (res types.Result) {
	return app.appConn.EchoSync(msg)
}

func (app *appConnQuery) InfoSync(req types.RequestInfo) (types.ResponseInfo, error) {
	return app.appConn.InfoSync(req)
}

func (app *appConnQuery) QuerySync(reqQuery types.RequestQuery) (types.ResponseQuery, error) {
	return app.appConn.QuerySync(reqQuery)
}
