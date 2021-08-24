package proxy

import (
	"context"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/pkg/abci"
)

//go:generate ../scripts/mockery_generate.sh AppConnConsensus|AppConnMempool|AppConnQuery|AppConnSnapshot

//----------------------------------------------------------------------------------------
// Enforce which abci msgs can be sent on a connection at the type level

type AppConnConsensus interface {
	SetResponseCallback(abcicli.Callback)
	Error() error

	InitChainSync(context.Context, abci.RequestInitChain) (*abci.ResponseInitChain, error)

	BeginBlockSync(context.Context, abci.RequestBeginBlock) (*abci.ResponseBeginBlock, error)
	DeliverTxAsync(context.Context, abci.RequestDeliverTx) (*abcicli.ReqRes, error)
	EndBlockSync(context.Context, abci.RequestEndBlock) (*abci.ResponseEndBlock, error)
	CommitSync(context.Context) (*abci.ResponseCommit, error)
}

type AppConnMempool interface {
	SetResponseCallback(abcicli.Callback)
	Error() error

	CheckTxAsync(context.Context, abci.RequestCheckTx) (*abcicli.ReqRes, error)
	CheckTxSync(context.Context, abci.RequestCheckTx) (*abci.ResponseCheckTx, error)

	FlushAsync(context.Context) (*abcicli.ReqRes, error)
	FlushSync(context.Context) error
}

type AppConnQuery interface {
	Error() error

	EchoSync(context.Context, string) (*abci.ResponseEcho, error)
	InfoSync(context.Context, abci.RequestInfo) (*abci.ResponseInfo, error)
	QuerySync(context.Context, abci.RequestQuery) (*abci.ResponseQuery, error)
}

type AppConnSnapshot interface {
	Error() error

	ListSnapshotsSync(context.Context, abci.RequestListSnapshots) (*abci.ResponseListSnapshots, error)
	OfferSnapshotSync(context.Context, abci.RequestOfferSnapshot) (*abci.ResponseOfferSnapshot, error)
	LoadSnapshotChunkSync(context.Context, abci.RequestLoadSnapshotChunk) (*abci.ResponseLoadSnapshotChunk, error)
	ApplySnapshotChunkSync(context.Context, abci.RequestApplySnapshotChunk) (*abci.ResponseApplySnapshotChunk, error)
}

//-----------------------------------------------------------------------------------------
// Implements AppConnConsensus (subset of abcicli.Client)

type appConnConsensus struct {
	appConn abcicli.Client
}

func NewAppConnConsensus(appConn abcicli.Client) AppConnConsensus {
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

func (app *appConnConsensus) InitChainSync(
	ctx context.Context,
	req abci.RequestInitChain,
) (*abci.ResponseInitChain, error) {
	return app.appConn.InitChainSync(ctx, req)
}

func (app *appConnConsensus) BeginBlockSync(
	ctx context.Context,
	req abci.RequestBeginBlock,
) (*abci.ResponseBeginBlock, error) {
	return app.appConn.BeginBlockSync(ctx, req)
}

func (app *appConnConsensus) DeliverTxAsync(ctx context.Context, req abci.RequestDeliverTx) (*abcicli.ReqRes, error) {
	return app.appConn.DeliverTxAsync(ctx, req)
}

func (app *appConnConsensus) EndBlockSync(
	ctx context.Context,
	req abci.RequestEndBlock,
) (*abci.ResponseEndBlock, error) {
	return app.appConn.EndBlockSync(ctx, req)
}

func (app *appConnConsensus) CommitSync(ctx context.Context) (*abci.ResponseCommit, error) {
	return app.appConn.CommitSync(ctx)
}

//------------------------------------------------
// Implements AppConnMempool (subset of abcicli.Client)

type appConnMempool struct {
	appConn abcicli.Client
}

func NewAppConnMempool(appConn abcicli.Client) AppConnMempool {
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

func (app *appConnMempool) FlushAsync(ctx context.Context) (*abcicli.ReqRes, error) {
	return app.appConn.FlushAsync(ctx)
}

func (app *appConnMempool) FlushSync(ctx context.Context) error {
	return app.appConn.FlushSync(ctx)
}

func (app *appConnMempool) CheckTxAsync(ctx context.Context, req abci.RequestCheckTx) (*abcicli.ReqRes, error) {
	return app.appConn.CheckTxAsync(ctx, req)
}

func (app *appConnMempool) CheckTxSync(ctx context.Context, req abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	return app.appConn.CheckTxSync(ctx, req)
}

//------------------------------------------------
// Implements AppConnQuery (subset of abcicli.Client)

type appConnQuery struct {
	appConn abcicli.Client
}

func NewAppConnQuery(appConn abcicli.Client) AppConnQuery {
	return &appConnQuery{
		appConn: appConn,
	}
}

func (app *appConnQuery) Error() error {
	return app.appConn.Error()
}

func (app *appConnQuery) EchoSync(ctx context.Context, msg string) (*abci.ResponseEcho, error) {
	return app.appConn.EchoSync(ctx, msg)
}

func (app *appConnQuery) InfoSync(ctx context.Context, req abci.RequestInfo) (*abci.ResponseInfo, error) {
	return app.appConn.InfoSync(ctx, req)
}

func (app *appConnQuery) QuerySync(ctx context.Context, reqQuery abci.RequestQuery) (*abci.ResponseQuery, error) {
	return app.appConn.QuerySync(ctx, reqQuery)
}

//------------------------------------------------
// Implements AppConnSnapshot (subset of abcicli.Client)

type appConnSnapshot struct {
	appConn abcicli.Client
}

func NewAppConnSnapshot(appConn abcicli.Client) AppConnSnapshot {
	return &appConnSnapshot{
		appConn: appConn,
	}
}

func (app *appConnSnapshot) Error() error {
	return app.appConn.Error()
}

func (app *appConnSnapshot) ListSnapshotsSync(
	ctx context.Context,
	req abci.RequestListSnapshots,
) (*abci.ResponseListSnapshots, error) {
	return app.appConn.ListSnapshotsSync(ctx, req)
}

func (app *appConnSnapshot) OfferSnapshotSync(
	ctx context.Context,
	req abci.RequestOfferSnapshot,
) (*abci.ResponseOfferSnapshot, error) {
	return app.appConn.OfferSnapshotSync(ctx, req)
}

func (app *appConnSnapshot) LoadSnapshotChunkSync(
	ctx context.Context,
	req abci.RequestLoadSnapshotChunk) (*abci.ResponseLoadSnapshotChunk, error) {
	return app.appConn.LoadSnapshotChunkSync(ctx, req)
}

func (app *appConnSnapshot) ApplySnapshotChunkSync(
	ctx context.Context,
	req abci.RequestApplySnapshotChunk) (*abci.ResponseApplySnapshotChunk, error) {
	return app.appConn.ApplySnapshotChunkSync(ctx, req)
}
