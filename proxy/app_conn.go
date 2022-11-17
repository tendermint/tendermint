package proxy

import (
	"context"
	"time"

	"github.com/go-kit/kit/metrics"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/types"
)

//go:generate ../scripts/mockery_generate.sh AppConnConsensus|AppConnMempool|AppConnQuery|AppConnSnapshot

//----------------------------------------------------------------------------------------
// Enforce which abci msgs can be sent on a connection at the type level

type AppConnConsensus interface {
	Error() error
	InitChain(context.Context, *types.RequestInitChain) (*types.ResponseInitChain, error)
	PrepareProposal(context.Context, *types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error)
	ProcessProposal(context.Context, *types.RequestProcessProposal) (*types.ResponseProcessProposal, error)
	FinalizeBlock(context.Context, *types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error)
	Commit(context.Context) (*types.ResponseCommit, error)
}

type AppConnMempool interface {
	SetResponseCallback(abcicli.Callback)
	Error() error

	CheckTx(context.Context, *types.RequestCheckTx) (*types.ResponseCheckTx, error)
	CheckTxAsync(context.Context, *types.RequestCheckTx) (*abcicli.ReqRes, error)
	Flush(context.Context) error
}

type AppConnQuery interface {
	Error() error

	Echo(context.Context, string) (*types.ResponseEcho, error)
	Info(context.Context, *types.RequestInfo) (*types.ResponseInfo, error)
	Query(context.Context, *types.RequestQuery) (*types.ResponseQuery, error)
}

type AppConnSnapshot interface {
	Error() error

	ListSnapshots(context.Context, *types.RequestListSnapshots) (*types.ResponseListSnapshots, error)
	OfferSnapshot(context.Context, *types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error)
	LoadSnapshotChunk(context.Context, *types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error)
	ApplySnapshotChunk(context.Context, *types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error)
}

//-----------------------------------------------------------------------------------------
// Implements AppConnConsensus (subset of abcicli.Client)

type appConnConsensus struct {
	metrics *Metrics
	appConn abcicli.Client
}

var _ AppConnConsensus = (*appConnConsensus)(nil)

func NewAppConnConsensus(appConn abcicli.Client, metrics *Metrics) AppConnConsensus {
	return &appConnConsensus{
		metrics: metrics,
		appConn: appConn,
	}
}

func (app *appConnConsensus) Error() error {
	return app.appConn.Error()
}

func (app *appConnConsensus) InitChain(ctx context.Context, req *types.RequestInitChain) (*types.ResponseInitChain, error) {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "init_chain"))()
	return app.appConn.InitChain(ctx, req)
}

func (app *appConnConsensus) PrepareProposal(ctx context.Context,
	req *types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "prepare_proposal"))()
	return app.appConn.PrepareProposal(ctx, req)
}

func (app *appConnConsensus) ProcessProposal(ctx context.Context, req *types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "process_proposal"))()
	return app.appConn.ProcessProposal(ctx, req)
}

func (app *appConnConsensus) FinalizeBlock(ctx context.Context, req *types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "deliver_tx", "type", "async"))()
	return app.appConn.FinalizeBlock(ctx, req)
}

func (app *appConnConsensus) Commit(ctx context.Context) (*types.ResponseCommit, error) {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "commit"))()
	return app.appConn.Commit(ctx, &types.RequestCommit{})
}

//------------------------------------------------
// Implements AppConnMempool (subset of abcicli.Client)

type appConnMempool struct {
	metrics *Metrics
	appConn abcicli.Client
}

func NewAppConnMempool(appConn abcicli.Client, metrics *Metrics) AppConnMempool {
	return &appConnMempool{
		metrics: metrics,
		appConn: appConn,
	}
}

func (app *appConnMempool) SetResponseCallback(cb abcicli.Callback) {
	app.appConn.SetResponseCallback(cb)
}

func (app *appConnMempool) Error() error {
	return app.appConn.Error()
}

func (app *appConnMempool) Flush(ctx context.Context) error {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "flush"))()
	return app.appConn.Flush(ctx)
}

func (app *appConnMempool) CheckTx(ctx context.Context, req *types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "check_tx", "type", "sync"))()
	return app.appConn.CheckTx(ctx, req)
}

func (app *appConnMempool) CheckTxAsync(ctx context.Context, req *types.RequestCheckTx) (*abcicli.ReqRes, error) {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "check_tx", "type", "async"))()
	return app.appConn.CheckTxAsync(ctx, req)
}

//------------------------------------------------
// Implements AppConnQuery (subset of abcicli.Client)

type appConnQuery struct {
	metrics *Metrics
	appConn abcicli.Client
}

func NewAppConnQuery(appConn abcicli.Client, metrics *Metrics) AppConnQuery {
	return &appConnQuery{
		metrics: metrics,
		appConn: appConn,
	}
}

func (app *appConnQuery) Error() error {
	return app.appConn.Error()
}

func (app *appConnQuery) Echo(ctx context.Context, msg string) (*types.ResponseEcho, error) {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "echo"))()
	return app.appConn.Echo(ctx, msg)
}

func (app *appConnQuery) Info(ctx context.Context, req *types.RequestInfo) (*types.ResponseInfo, error) {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "info"))()
	return app.appConn.Info(ctx, req)
}

func (app *appConnQuery) Query(ctx context.Context, req *types.RequestQuery) (*types.ResponseQuery, error) {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "query"))()
	return app.appConn.Query(ctx, req)
}

//------------------------------------------------
// Implements AppConnSnapshot (subset of abcicli.Client)

type appConnSnapshot struct {
	metrics *Metrics
	appConn abcicli.Client
}

func NewAppConnSnapshot(appConn abcicli.Client, metrics *Metrics) AppConnSnapshot {
	return &appConnSnapshot{
		metrics: metrics,
		appConn: appConn,
	}
}

func (app *appConnSnapshot) Error() error {
	return app.appConn.Error()
}

func (app *appConnSnapshot) ListSnapshots(ctx context.Context, req *types.RequestListSnapshots) (*types.ResponseListSnapshots, error) {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "list_snapshots"))()
	return app.appConn.ListSnapshots(ctx, req)
}

func (app *appConnSnapshot) OfferSnapshot(ctx context.Context, req *types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "offer_snapshot"))()
	return app.appConn.OfferSnapshot(ctx, req)
}

func (app *appConnSnapshot) LoadSnapshotChunk(ctx context.Context, req *types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "load_snapshot_chunk"))()
	return app.appConn.LoadSnapshotChunk(ctx, req)
}

func (app *appConnSnapshot) ApplySnapshotChunk(ctx context.Context, req *types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {
	defer addTimeSample(app.metrics.MethodTimingSeconds.With("method", "apply_snapshot_chunk"))()
	return app.appConn.ApplySnapshotChunk(ctx, req)
}

// addTimeSample returns a function that, when called, adds an observation to m.
// The observation added to m is the number of seconds ellapsed since addTimeSample
// was initially called. addTimeSample is meant to be called in a defer to calculate
// the amount of time a function takes to complete.
func addTimeSample(m metrics.Histogram) func() {
	start := time.Now()
	return func() { m.Observe(time.Since(start).Seconds()) }
}
