package proxy

import (
	"context"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/go-kit/kit/metrics"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	e2e "github.com/tendermint/tendermint/test/e2e/app"
)

// ClientFactory returns a client object, which will create a local
// client if addr is one of: 'kvstore', 'persistent_kvstore', 'e2e',
// or 'noop', otherwise - a remote client.
//
// The Closer is a noop except for persistent_kvstore applications,
// which will clean up the store.
func ClientFactory(logger log.Logger, addr, transport, dbDir string) (abciclient.Client, io.Closer, error) {
	switch addr {
	case "kvstore":
		return abciclient.NewLocalClient(logger, kvstore.NewApplication()), noopCloser{}, nil
	case "persistent_kvstore":
		app := kvstore.NewPersistentKVStoreApplication(logger, dbDir)
		return abciclient.NewLocalClient(logger, app), app, nil
	case "e2e":
		app, err := e2e.NewApplication(e2e.DefaultConfig(dbDir))
		if err != nil {
			return nil, noopCloser{}, err
		}
		return abciclient.NewLocalClient(logger, app), noopCloser{}, nil
	case "noop":
		return abciclient.NewLocalClient(logger, types.NewBaseApplication()), noopCloser{}, nil
	default:
		const mustConnect = false // loop retrying
		client, err := abciclient.NewClient(logger, addr, transport, mustConnect)
		if err != nil {
			return nil, noopCloser{}, err
		}

		return client, noopCloser{}, nil
	}
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }

// proxyClient provides the application connection.
type proxyClient struct {
	service.BaseService
	logger log.Logger

	client  abciclient.Client
	metrics *Metrics
}

// New creates a proxy application interface.
func New(client abciclient.Client, logger log.Logger, metrics *Metrics) abciclient.Client {
	conn := &proxyClient{
		logger:  logger,
		metrics: metrics,
		client:  client,
	}
	conn.BaseService = *service.NewBaseService(logger, "proxyClient", conn)
	return conn
}

func (app *proxyClient) OnStop()      { tryCallStop(app.client) }
func (app *proxyClient) Error() error { return app.client.Error() }

func tryCallStop(client abciclient.Client) {
	if c, ok := client.(interface{ Stop() }); ok {
		c.Stop()
	}
}

func (app *proxyClient) OnStart(ctx context.Context) error {
	var err error
	defer func() {
		if err != nil {
			tryCallStop(app.client)
		}
	}()

	// Kill Tendermint if the ABCI application crashes.
	go func() {
		if !app.client.IsRunning() {
			return
		}
		app.client.Wait()
		if ctx.Err() != nil {
			return
		}

		if err := app.client.Error(); err != nil {
			app.logger.Error("client connection terminated. Did the application crash? Please restart tendermint",
				"err", err)

			if killErr := kill(); killErr != nil {
				app.logger.Error("Failed to kill this process - please do so manually",
					"err", killErr)
			}
		}

	}()

	return app.client.Start(ctx)
}

func kill() error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}

	return p.Signal(syscall.SIGABRT)
}

func (app *proxyClient) InitChain(ctx context.Context, req *types.RequestInitChain) (*types.ResponseInitChain, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "init_chain", "type", "sync"))()
	return app.client.InitChain(ctx, req)
}

func (app *proxyClient) PrepareProposal(ctx context.Context, req *types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "prepare_proposal", "type", "sync"))()
	return app.client.PrepareProposal(ctx, req)
}

func (app *proxyClient) ProcessProposal(ctx context.Context, req *types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "process_proposal", "type", "sync"))()
	return app.client.ProcessProposal(ctx, req)
}

func (app *proxyClient) ExtendVote(ctx context.Context, req *types.RequestExtendVote) (*types.ResponseExtendVote, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "extend_vote", "type", "sync"))()
	return app.client.ExtendVote(ctx, req)
}

func (app *proxyClient) VerifyVoteExtension(ctx context.Context, req *types.RequestVerifyVoteExtension) (*types.ResponseVerifyVoteExtension, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "verify_vote_extension", "type", "sync"))()
	return app.client.VerifyVoteExtension(ctx, req)
}

func (app *proxyClient) FinalizeBlock(ctx context.Context, req *types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
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

func (app *proxyClient) CheckTx(ctx context.Context, req *types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "check_tx", "type", "sync"))()
	return app.client.CheckTx(ctx, req)
}

func (app *proxyClient) Echo(ctx context.Context, msg string) (*types.ResponseEcho, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "echo", "type", "sync"))()
	return app.client.Echo(ctx, msg)
}

func (app *proxyClient) Info(ctx context.Context, req *types.RequestInfo) (*types.ResponseInfo, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "info", "type", "sync"))()
	return app.client.Info(ctx, req)
}

func (app *proxyClient) Query(ctx context.Context, req *types.RequestQuery) (*types.ResponseQuery, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "query", "type", "sync"))()
	return app.client.Query(ctx, req)
}

func (app *proxyClient) ListSnapshots(ctx context.Context, req *types.RequestListSnapshots) (*types.ResponseListSnapshots, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "list_snapshots", "type", "sync"))()
	return app.client.ListSnapshots(ctx, req)
}

func (app *proxyClient) OfferSnapshot(ctx context.Context, req *types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "offer_snapshot", "type", "sync"))()
	return app.client.OfferSnapshot(ctx, req)
}

func (app *proxyClient) LoadSnapshotChunk(ctx context.Context, req *types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {
	defer addTimeSample(app.metrics.MethodTiming.With("method", "load_snapshot_chunk", "type", "sync"))()
	return app.client.LoadSnapshotChunk(ctx, req)
}

func (app *proxyClient) ApplySnapshotChunk(ctx context.Context, req *types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {
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
