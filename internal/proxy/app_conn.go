package proxy

import (
	"context"
	"time"

	"github.com/go-kit/kit/metrics"

	"github.com/tendermint/tendermint/abci/types"
)

func (app *proxyClient) InitChain(ctx context.Context, req types.RequestInitChain) (*types.ResponseInitChain, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "init_chain", "type", "sync"))()
	return app.client.InitChain(ctx, req)
}

func (app *proxyClient) PrepareProposal(ctx context.Context, req types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "prepare_proposal", "type", "sync"))()
	return app.client.PrepareProposal(ctx, req)
}

func (app *proxyClient) ProcessProposal(ctx context.Context, req types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "process_proposal", "type", "sync"))()
	return app.client.ProcessProposal(ctx, req)
}

func (app *proxyClient) ExtendVote(ctx context.Context, req types.RequestExtendVote) (*types.ResponseExtendVote, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "extend_vote", "type", "sync"))()
	return app.client.ExtendVote(ctx, req)
}

func (app *proxyClient) VerifyVoteExtension(ctx context.Context, req types.RequestVerifyVoteExtension) (*types.ResponseVerifyVoteExtension, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "verify_vote_extension", "type", "sync"))()
	return app.client.VerifyVoteExtension(ctx, req)
}

func (app *proxyClient) FinalizeBlock(ctx context.Context, req types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "finalize_block", "type", "sync"))()
	return app.client.FinalizeBlock(ctx, req)
}

func (app *proxyClient) Commit(ctx context.Context) (*types.ResponseCommit, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "commit", "type", "sync"))()
	return app.client.Commit(ctx)
}

func (app *proxyClient) Flush(ctx context.Context) error {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "flush", "type", "sync"))()
	return app.client.Flush(ctx)
}

func (app *proxyClient) CheckTx(ctx context.Context, req types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "check_tx", "type", "sync"))()
	return app.client.CheckTx(ctx, req)
}

func (app *proxyClient) Echo(ctx context.Context, msg string) (*types.ResponseEcho, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "echo", "type", "sync"))()
	return app.client.Echo(ctx, msg)
}

func (app *proxyClient) Info(ctx context.Context, req types.RequestInfo) (*types.ResponseInfo, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "info", "type", "sync"))()
	return app.client.Info(ctx, req)
}

func (app *proxyClient) Query(ctx context.Context, reqQuery types.RequestQuery) (*types.ResponseQuery, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "query", "type", "sync"))()
	return app.client.Query(ctx, reqQuery)
}

func (app *proxyClient) ListSnapshots(ctx context.Context, req types.RequestListSnapshots) (*types.ResponseListSnapshots, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "list_snapshots", "type", "sync"))()
	return app.client.ListSnapshots(ctx, req)
}

func (app *proxyClient) OfferSnapshot(ctx context.Context, req types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "offer_snapshot", "type", "sync"))()
	return app.client.OfferSnapshot(ctx, req)
}

func (app *proxyClient) LoadSnapshotChunk(ctx context.Context, req types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "load_snapshot_chunk", "type", "sync"))()
	return app.client.LoadSnapshotChunk(ctx, req)
}

func (app *proxyClient) ApplySnapshotChunk(ctx context.Context, req types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "apply_snapshot_chunk", "type", "sync"))()
	return app.client.ApplySnapshotChunk(ctx, req)
}

// addTimeSample returns a function that, when called, adds an observation to m.
// The observation added to m is the number of seconds ellapsed since addTimeSample
// was initially called. addTimeSample is meant to be called in a defer to calculate
// the amount of time a function takes to complete.
func addTimeSample(m metrics.Histogram) func() {
	start := time.Now()
	return func() { m.Observe(time.Since(start).Seconds()) }
}
