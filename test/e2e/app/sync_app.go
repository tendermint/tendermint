package app

import (
	"sync"

	abci "github.com/tendermint/tendermint/abci/types"
)

// SyncApplication wraps an Application, managing its own synchronization. This
// allows it to be called from an unsynchronized local client, as it is
// implemented in a thread-safe way.
type SyncApplication struct {
	mtx sync.RWMutex
	app abci.Application
}

var _ abci.Application = (*SyncApplication)(nil)

func NewSyncApplication(cfg *Config) (abci.Application, error) {
	app, err := NewApplication(cfg)
	if err != nil {
		return nil, err
	}
	return &SyncApplication{
		app: app,
	}, nil
}

func (app *SyncApplication) Info(req abci.RequestInfo) abci.ResponseInfo {
	app.mtx.RLock()
	defer app.mtx.RUnlock()
	return app.app.Info(req)
}

func (app *SyncApplication) InitChain(req abci.RequestInitChain) abci.ResponseInitChain {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.app.InitChain(req)
}

func (app *SyncApplication) CheckTx(req abci.RequestCheckTx) abci.ResponseCheckTx {
	app.mtx.RLock()
	defer app.mtx.RUnlock()
	return app.app.CheckTx(req)
}

func (app *SyncApplication) PrepareProposal(req abci.RequestPrepareProposal) abci.ResponsePrepareProposal {
	// app.app.PrepareProposal does not modify state
	app.mtx.RLock()
	defer app.mtx.RUnlock()
	return app.app.PrepareProposal(req)
}

func (app *SyncApplication) ProcessProposal(req abci.RequestProcessProposal) abci.ResponseProcessProposal {
	// app.app.ProcessProposal does not modify state
	app.mtx.RLock()
	defer app.mtx.RUnlock()
	return app.app.ProcessProposal(req)
}

func (app *SyncApplication) DeliverTx(req abci.RequestDeliverTx) abci.ResponseDeliverTx {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.app.DeliverTx(req)
}

func (app *SyncApplication) BeginBlock(req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.app.BeginBlock(req)
}

func (app *SyncApplication) EndBlock(req abci.RequestEndBlock) abci.ResponseEndBlock {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.app.EndBlock(req)
}

func (app *SyncApplication) Commit() abci.ResponseCommit {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.app.Commit()
}

func (app *SyncApplication) Query(req abci.RequestQuery) abci.ResponseQuery {
	app.mtx.RLock()
	defer app.mtx.RUnlock()
	return app.app.Query(req)
}

func (app *SyncApplication) ApplySnapshotChunk(req abci.RequestApplySnapshotChunk) abci.ResponseApplySnapshotChunk {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.app.ApplySnapshotChunk(req)
}

func (app *SyncApplication) ListSnapshots(req abci.RequestListSnapshots) abci.ResponseListSnapshots {
	// Calls app.snapshots.List(), which is thread-safe.
	return app.app.ListSnapshots(req)
}

func (app *SyncApplication) LoadSnapshotChunk(req abci.RequestLoadSnapshotChunk) abci.ResponseLoadSnapshotChunk {
	// Calls app.snapshots.LoadChunk, which is thread-safe.
	return app.app.LoadSnapshotChunk(req)
}

func (app *SyncApplication) OfferSnapshot(req abci.RequestOfferSnapshot) abci.ResponseOfferSnapshot {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.app.OfferSnapshot(req)
}
