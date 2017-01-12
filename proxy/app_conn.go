package proxy

import (
	tmspcli "github.com/tendermint/tmsp/client"
	"github.com/tendermint/tmsp/types"
)

//----------------------------------------------------------------------------------------
// Enforce which tmsp msgs can be sent on a connection at the type level

type AppConnConsensus interface {
	SetResponseCallback(tmspcli.Callback)
	Error() error

	InitChainSync(validators []*types.Validator) (err error)

	BeginBlockSync(hash []byte, header *types.Header) (err error)
	AppendTxAsync(tx []byte) *tmspcli.ReqRes
	EndBlockSync(height uint64) (changedValidators []*types.Validator, err error)
	CommitSync() (res types.Result)
}

type AppConnMempool interface {
	SetResponseCallback(tmspcli.Callback)
	Error() error

	CheckTxAsync(tx []byte) *tmspcli.ReqRes

	FlushAsync() *tmspcli.ReqRes
	FlushSync() error
}

type AppConnQuery interface {
	Error() error

	EchoSync(string) (res types.Result)
	InfoSync() (types.Result, *types.TMSPInfo, *types.LastBlockInfo, *types.ConfigInfo)
	QuerySync(tx []byte) (res types.Result)
	ProofSync(key []byte, blockHeight int64) (res types.Result)

	//	SetOptionSync(key string, value string) (res types.Result)
}

//-----------------------------------------------------------------------------------------
// Implements AppConnConsensus (subset of tmspcli.Client)

type appConnConsensus struct {
	appConn tmspcli.Client
}

func NewAppConnConsensus(appConn tmspcli.Client) *appConnConsensus {
	return &appConnConsensus{
		appConn: appConn,
	}
}

func (app *appConnConsensus) SetResponseCallback(cb tmspcli.Callback) {
	app.appConn.SetResponseCallback(cb)
}

func (app *appConnConsensus) Error() error {
	return app.appConn.Error()
}

func (app *appConnConsensus) InitChainSync(validators []*types.Validator) (err error) {
	return app.appConn.InitChainSync(validators)
}

func (app *appConnConsensus) BeginBlockSync(hash []byte, header *types.Header) (err error) {
	return app.appConn.BeginBlockSync(hash, header)
}

func (app *appConnConsensus) AppendTxAsync(tx []byte) *tmspcli.ReqRes {
	return app.appConn.AppendTxAsync(tx)
}

func (app *appConnConsensus) EndBlockSync(height uint64) (changedValidators []*types.Validator, err error) {
	return app.appConn.EndBlockSync(height)
}

func (app *appConnConsensus) CommitSync() (res types.Result) {
	return app.appConn.CommitSync()
}

//------------------------------------------------
// Implements AppConnMempool (subset of tmspcli.Client)

type appConnMempool struct {
	appConn tmspcli.Client
}

func NewAppConnMempool(appConn tmspcli.Client) *appConnMempool {
	return &appConnMempool{
		appConn: appConn,
	}
}

func (app *appConnMempool) SetResponseCallback(cb tmspcli.Callback) {
	app.appConn.SetResponseCallback(cb)
}

func (app *appConnMempool) Error() error {
	return app.appConn.Error()
}

func (app *appConnMempool) FlushAsync() *tmspcli.ReqRes {
	return app.appConn.FlushAsync()
}

func (app *appConnMempool) FlushSync() error {
	return app.appConn.FlushSync()
}

func (app *appConnMempool) CheckTxAsync(tx []byte) *tmspcli.ReqRes {
	return app.appConn.CheckTxAsync(tx)
}

//------------------------------------------------
// Implements AppConnQuery (subset of tmspcli.Client)

type appConnQuery struct {
	appConn tmspcli.Client
}

func NewAppConnQuery(appConn tmspcli.Client) *appConnQuery {
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

func (app *appConnQuery) InfoSync() (types.Result, *types.TMSPInfo, *types.LastBlockInfo, *types.ConfigInfo) {
	return app.appConn.InfoSync()
}

func (app *appConnQuery) QuerySync(tx []byte) (res types.Result) {
	return app.appConn.QuerySync(tx)
}

func (app *appConnQuery) ProofSync(key []byte, blockHeight int64) (res types.Result) {
	return app.appConn.ProofSync(key, blockHeight)
}
