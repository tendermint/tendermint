package proxy

import (
	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/types"
)

//----------------------------------------------------------------------------------------
// Enforce which abci msgs can be sent on a connection at the type level

type AppConnConsensus interface {
	SetResponseCallback(abcicli.Callback)
	Error() error

	InitChainSync(types.RequestInitChain) (*types.ResponseInitChain, error)

	BeginBlockSync(types.RequestBeginBlock) (*types.ResponseBeginBlock, error)
	DeliverTxAsync(tx []byte) *abcicli.ReqRes
	EndBlockSync(types.RequestEndBlock) (*types.ResponseEndBlock, error)
	CommitSync() (*types.ResponseCommit, error)
}

type AppConnMempool interface {
	SetResponseCallback(abcicli.Callback)
	Error() error

	CheckTxAsync(tx []byte) *abcicli.ReqRes

	FlushAsync() *abcicli.ReqRes
	FlushSync() error
}

type AppConnState interface {
	SetResponseCallback(abcicli.Callback)
	Error() error

	LatestSnapshot() (height int64, numKeys []int64, err error)
	ReadSnapshotChunk(height int64, startIndex, endIndex int64) (chunk [][]byte, err error)
	StartRecovery(height int64, numKeys []int64) error
	WriteRecoveryChunk(chunk [][]byte) error
	EndRecovery(height int64) error
}

type AppConnQuery interface {
	Error() error

	EchoSync(string) (*types.ResponseEcho, error)
	InfoSync(types.RequestInfo) (*types.ResponseInfo, error)
	QuerySync(types.RequestQuery) (*types.ResponseQuery, error)

	//	SetOptionSync(key string, value string) (res types.Result)
}

//-----------------------------------------------------------------------------------------
// Implements AppConnConsensus (subset of abcicli.Client)

type appConnState struct {
	appConn abcicli.Client
}

func NewAppConnState(appConn abcicli.Client) *appConnState {
	return &appConnState{
		appConn: appConn,
	}
}

func (app *appConnState) SetResponseCallback(cb abcicli.Callback) {
	app.appConn.SetResponseCallback(cb)
}

func (app *appConnState) Error() error {
	return app.appConn.Error()
}

func (app *appConnState) LatestSnapshot() (height int64, numKeys []int64, err error) {
	return app.appConn.LatestSnapshot()
}

func (app *appConnState) ReadSnapshotChunk(height int64, startIndex, endIndex int64) (chunk [][]byte, err error) {
	return app.appConn.ReadSnapshotChunk(height, startIndex, endIndex)
}

func (app *appConnState) StartRecovery(height int64, numKeys []int64) error {
	return app.appConn.StartRecovery(height, numKeys)
}

func (app *appConnState) WriteRecoveryChunk(chunk [][]byte) error {
	return app.appConn.WriteRecoveryChunk(chunk)
}

func (app *appConnState) EndRecovery(height int64) error {
	return app.appConn.EndRecovery(height)
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

func (app *appConnConsensus) InitChainSync(req types.RequestInitChain) (*types.ResponseInitChain, error) {
	return app.appConn.InitChainSync(req)
}

func (app *appConnConsensus) BeginBlockSync(req types.RequestBeginBlock) (*types.ResponseBeginBlock, error) {
	return app.appConn.BeginBlockSync(req)
}

func (app *appConnConsensus) DeliverTxAsync(tx []byte) *abcicli.ReqRes {
	return app.appConn.DeliverTxAsync(tx)
}

func (app *appConnConsensus) EndBlockSync(req types.RequestEndBlock) (*types.ResponseEndBlock, error) {
	return app.appConn.EndBlockSync(req)
}

func (app *appConnConsensus) CommitSync() (*types.ResponseCommit, error) {
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

func (app *appConnQuery) EchoSync(msg string) (*types.ResponseEcho, error) {
	return app.appConn.EchoSync(msg)
}

func (app *appConnQuery) InfoSync(req types.RequestInfo) (*types.ResponseInfo, error) {
	return app.appConn.InfoSync(req)
}

func (app *appConnQuery) QuerySync(reqQuery types.RequestQuery) (*types.ResponseQuery, error) {
	return app.appConn.QuerySync(reqQuery)
}
