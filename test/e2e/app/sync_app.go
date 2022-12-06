package app

import (
	"context"
	"sync"

	abci "github.com/tendermint/tendermint/abci/types"
)

// SyncApplication wraps the e2e Application, managing its own synchronization. This
// allows it to be called from an unsynchronized local client, as it is
// implemented in a thread-safe way.
type SyncApplication struct {
	mtx sync.RWMutex
	app *Application
}

var _ abci.Application = (*SyncApplication)(nil)

func NewSyncApplication(cfg *Config) (abci.Application, error) {
	app, err := NewApplication(cfg)
	if err != nil {
		return nil, err
	}
	return &SyncApplication{
		app: app.(*Application),
	}, nil
}

func (app *SyncApplication) Info(ctx context.Context, req *abci.RequestInfo) (*abci.ResponseInfo, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()
	return app.app.Info(ctx, req)
}

func (app *SyncApplication) InitChain(ctx context.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.app.InitChain(ctx, req)
}

func (app *SyncApplication) CheckTx(ctx context.Context, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()
	return app.app.CheckTx(ctx, req)
}

func (app *SyncApplication) PrepareProposal(ctx context.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	// app.app.PrepareProposal does not modify state
	app.mtx.RLock()
	defer app.mtx.RUnlock()
	return app.app.PrepareProposal(ctx, req)
}

func (app *SyncApplication) ProcessProposal(ctx context.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	// app.app.ProcessProposal does not modify state
	app.mtx.RLock()
	defer app.mtx.RUnlock()
	return app.app.ProcessProposal(ctx, req)
}

func (app *SyncApplication) ExtendVote(ctx context.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.app.ExtendVote(ctx, req)
}

func (app *SyncApplication) VerifyVoteExtension(ctx context.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.app.VerifyVoteExtension(ctx, req)
}

func (app *SyncApplication) FinalizeBlock(ctx context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.app.FinalizeBlock(ctx, req)
}

func (app *SyncApplication) Commit(ctx context.Context, req *abci.RequestCommit) (*abci.ResponseCommit, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.app.Commit(ctx, req)
}

func (app *SyncApplication) Query(ctx context.Context, req *abci.RequestQuery) (*abci.ResponseQuery, error) {
	app.mtx.RLock()
	defer app.mtx.RUnlock()
	return app.app.Query(ctx, req)
}

func (app *SyncApplication) ApplySnapshotChunk(ctx context.Context, req *abci.RequestApplySnapshotChunk) (*abci.ResponseApplySnapshotChunk, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.app.ApplySnapshotChunk(ctx, req)
}

func (app *SyncApplication) ListSnapshots(ctx context.Context, req *abci.RequestListSnapshots) (*abci.ResponseListSnapshots, error) {
	// Calls app.snapshots.List(), which is thread-safe.
	return app.app.ListSnapshots(ctx, req)
}

func (app *SyncApplication) LoadSnapshotChunk(ctx context.Context, req *abci.RequestLoadSnapshotChunk) (*abci.ResponseLoadSnapshotChunk, error) {
	// Calls app.snapshots.LoadChunk, which is thread-safe.
	return app.app.LoadSnapshotChunk(ctx, req)
}

func (app *SyncApplication) OfferSnapshot(ctx context.Context, req *abci.RequestOfferSnapshot) (*abci.ResponseOfferSnapshot, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.app.OfferSnapshot(ctx, req)
}
