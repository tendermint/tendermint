package proxy

import (
	abcixcli "github.com/tendermint/tendermint/abcix/client"
	xtypes "github.com/tendermint/tendermint/abcix/types"
)

//go:generate mockery -case underscore -name AppConnConsensus|AppConnMempool|AppConnQuery|AppConnSnapshot

//----------------------------------------------------------------------------------------
// Enforce which abci msgs can be sent on a connection at the type level

type AppConnConsensus interface {
	SetResponseCallback(abcixcli.Callback)
	Error() error

	CreateBlockSync(xtypes.RequestCreateBlock, *xtypes.MempoolIter) (*xtypes.ResponseCreateBlock, error)
	InitChainSync(xtypes.RequestInitChain) (*xtypes.ResponseInitChain, error)
	DeliverBlockSync(xtypes.RequestDeliverBlock) (*xtypes.ResponseDeliverBlock, error)
	CheckBlockSync(xtypes.RequestCheckBlock) (*xtypes.ResponseCheckBlock, error)
	CommitSync() (*xtypes.ResponseCommit, error)
}

type AppConnMempool interface {
	SetResponseCallback(abcixcli.Callback)
	Error() error

	CheckTxAsync(xtypes.RequestCheckTx) *abcixcli.ReqRes
	CheckTxSync(xtypes.RequestCheckTx) (*xtypes.ResponseCheckTx, error)

	FlushAsync() *abcixcli.ReqRes
	FlushSync() error
}

type AppConnQuery interface {
	Error() error

	EchoSync(string) (*xtypes.ResponseEcho, error)
	InfoSync(xtypes.RequestInfo) (*xtypes.ResponseInfo, error)
	QuerySync(xtypes.RequestQuery) (*xtypes.ResponseQuery, error)
}

type AppConnSnapshot interface {
	Error() error

	ListSnapshotsSync(xtypes.RequestListSnapshots) (*xtypes.ResponseListSnapshots, error)
	OfferSnapshotSync(xtypes.RequestOfferSnapshot) (*xtypes.ResponseOfferSnapshot, error)
	LoadSnapshotChunkSync(xtypes.RequestLoadSnapshotChunk) (*xtypes.ResponseLoadSnapshotChunk, error)
	ApplySnapshotChunkSync(xtypes.RequestApplySnapshotChunk) (*xtypes.ResponseApplySnapshotChunk, error)
}

//-----------------------------------------------------------------------------------------
// Implements AppConnConsensus (subset of abcixcli.Client)

type appConnConsensus struct {
	appConn abcixcli.Client
}

func NewAppConnConsensus(appConn abcixcli.Client) AppConnConsensus {
	return &appConnConsensus{
		appConn: appConn,
	}
}

func (app *appConnConsensus) SetResponseCallback(cb abcixcli.Callback) {
	app.appConn.SetResponseCallback(cb)
}

func (app *appConnConsensus) Error() error {
	return app.appConn.Error()
}

func (app *appConnConsensus) CreateBlockSync(
	req xtypes.RequestCreateBlock,
	mempool *xtypes.MempoolIter,
) (*xtypes.ResponseCreateBlock, error) {
	return app.appConn.CreateBlockSync(req, mempool)
}

func (app *appConnConsensus) InitChainSync(req xtypes.RequestInitChain) (*xtypes.ResponseInitChain, error) {
	return app.appConn.InitChainSync(req)
}

func (app *appConnConsensus) DeliverBlockSync(req xtypes.RequestDeliverBlock) (*xtypes.ResponseDeliverBlock, error) {
	return app.appConn.DeliverBlockSync(req)
}

func (app *appConnConsensus) CommitSync() (*xtypes.ResponseCommit, error) {
	return app.appConn.CommitSync()
}

func (app *appConnConsensus) CheckBlockSync(req xtypes.RequestCheckBlock) (*xtypes.ResponseCheckBlock, error) {
	return app.appConn.CheckBlockSync(req)
}

//------------------------------------------------
// Implements AppConnMempool (subset of abcicli.Client)

type appConnMempool struct {
	appConn abcixcli.Client
}

func NewAppConnMempool(appConn abcixcli.Client) AppConnMempool {
	return &appConnMempool{
		appConn: appConn,
	}
}

func (app *appConnMempool) SetResponseCallback(cb abcixcli.Callback) {
	app.appConn.SetResponseCallback(cb)
}

func (app *appConnMempool) Error() error {
	return app.appConn.Error()
}

func (app *appConnMempool) FlushAsync() *abcixcli.ReqRes {
	return app.appConn.FlushAsync()
}

func (app *appConnMempool) FlushSync() error {
	return app.appConn.FlushSync()
}

func (app *appConnMempool) CheckTxAsync(req xtypes.RequestCheckTx) *abcixcli.ReqRes {
	return app.appConn.CheckTxAsync(req)
}

func (app *appConnMempool) CheckTxSync(req xtypes.RequestCheckTx) (*xtypes.ResponseCheckTx, error) {
	return app.appConn.CheckTxSync(req)
}

//------------------------------------------------
// Implements AppConnQuery (subset of abcicli.Client)

type appConnQuery struct {
	appConn abcixcli.Client
}

func NewAppConnQuery(appConn abcixcli.Client) AppConnQuery {
	return &appConnQuery{
		appConn: appConn,
	}
}

func (app *appConnQuery) Error() error {
	return app.appConn.Error()
}

func (app *appConnQuery) EchoSync(msg string) (*xtypes.ResponseEcho, error) {
	return app.appConn.EchoSync(msg)
}

func (app *appConnQuery) InfoSync(req xtypes.RequestInfo) (*xtypes.ResponseInfo, error) {
	return app.appConn.InfoSync(req)
}

func (app *appConnQuery) QuerySync(reqQuery xtypes.RequestQuery) (*xtypes.ResponseQuery, error) {
	return app.appConn.QuerySync(reqQuery)
}

//------------------------------------------------
// Implements AppConnSnapshot (subset of abcicli.Client)

type appConnSnapshot struct {
	appConn abcixcli.Client
}

func NewAppConnSnapshot(appConn abcixcli.Client) AppConnSnapshot {
	return &appConnSnapshot{
		appConn: appConn,
	}
}

func (app *appConnSnapshot) Error() error {
	return app.appConn.Error()
}

func (app *appConnSnapshot) ListSnapshotsSync(req xtypes.RequestListSnapshots) (*xtypes.ResponseListSnapshots, error) {
	return app.appConn.ListSnapshotsSync(req)
}

func (app *appConnSnapshot) OfferSnapshotSync(req xtypes.RequestOfferSnapshot) (*xtypes.ResponseOfferSnapshot, error) {
	return app.appConn.OfferSnapshotSync(req)
}

func (app *appConnSnapshot) LoadSnapshotChunkSync(
	req xtypes.RequestLoadSnapshotChunk) (*xtypes.ResponseLoadSnapshotChunk, error) {
	return app.appConn.LoadSnapshotChunkSync(req)
}

func (app *appConnSnapshot) ApplySnapshotChunkSync(
	req xtypes.RequestApplySnapshotChunk) (*xtypes.ResponseApplySnapshotChunk, error) {
	return app.appConn.ApplySnapshotChunkSync(req)
}
