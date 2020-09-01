package iavlstore

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosmos/iavl"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/example/iavlstore/snapshots"
	snapshottypes "github.com/tendermint/tendermint/abci/example/iavlstore/snapshots/types"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/version"
)

var _ abci.Application = (*Application)(nil)

type Application struct {
	abci.BaseApplication
	store           *iavl.MutableTree
	snapshotManager *snapshots.Manager
	logger          log.Logger
}

func NewApplication(dataDir string) *Application {
	// Set up IAVL store
	db, err := dbm.NewGoLevelDB("iavlstore", dataDir)
	if err != nil {
		panic(err)
	}
	store, err := iavl.NewMutableTree(db, 0)
	if err != nil {
		panic(err)
	}
	_, err = store.Load()
	if err != nil {
		panic(err)
	}

	// Set up snapshot storage
	snapshotDir := filepath.Join(dataDir, "snapshots")
	snapshotDB, err := dbm.NewGoLevelDB("metadata", snapshotDir)
	if err != nil {
		panic(err)
	}
	snapshotStore, err := snapshots.NewStore(snapshotDB, snapshotDir)
	if err != nil {
		panic(err)
	}

	return &Application{
		store:           store,
		snapshotManager: snapshots.NewManager(snapshotStore, &snapshots.IAVLSnapshotter{MutableTree: store}),
		logger:          log.NewTMLogger(log.NewSyncWriter(os.Stdout)),
	}
}

func (app *Application) Info(req abci.RequestInfo) (resInfo abci.ResponseInfo) {
	return abci.ResponseInfo{
		Data:             fmt.Sprintf(`{"size":%v}`, app.store.Size()),
		Version:          version.ABCIVersion,
		AppVersion:       1,
		LastBlockHeight:  app.store.Version(),
		LastBlockAppHash: app.store.Hash(),
	}
}

// parseTx parses a tx in 'key=value' format into a key and value.
func parseTx(tx []byte) ([]byte, []byte, error) {
	parts := bytes.Split(tx, []byte("="))
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("invalid tx format: %q", string(tx))
	}
	if len(parts[0]) == 0 {
		return nil, nil, errors.New("key cannot be empty")
	}
	return parts[0], parts[1], nil
}

func (app *Application) CheckTx(req abci.RequestCheckTx) abci.ResponseCheckTx {
	_, _, err := parseTx(req.Tx)
	if err != nil {
		return abci.ResponseCheckTx{
			Code: code.CodeTypeEncodingError,
			Log:  err.Error(),
		}
	}
	return abci.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}
}

func (app *Application) DeliverTx(req abci.RequestDeliverTx) abci.ResponseDeliverTx {
	key, value, err := parseTx(req.Tx)
	if err != nil {
		panic(err)
	}
	app.store.Set(key, value)
	return abci.ResponseDeliverTx{Code: code.CodeTypeOK}
}

func (app *Application) Commit() abci.ResponseCommit {
	hash, height, err := app.store.SaveVersion()
	if err != nil {
		panic(err)
	}

	// Take asynchronous state sync snapshot every 10 blocks
	if height%10 == 0 {
		go func() {
			app.logger.Info("Creating state snapshot", "height", height)
			snapshot, err := app.snapshotManager.Create(uint64(height))
			if err != nil {
				app.logger.Error("Failed to create state snapshot", "height", height, "err", err)
				return
			}
			app.logger.Info("Completed state snapshot", "height", height, "format", snapshot.Format)
		}()
	}

	return abci.ResponseCommit{Data: hash}
}

func (app *Application) Query(req abci.RequestQuery) abci.ResponseQuery {
	_, value := app.store.Get(req.Data)
	return abci.ResponseQuery{
		Height: app.store.Version(),
		Key:    req.Data,
		Value:  value,
	}
}

// ListSnapshots implements the ABCI interface. It delegates to app.snapshotManager if set.
func (app *Application) ListSnapshots(req abci.RequestListSnapshots) abci.ResponseListSnapshots {
	resp := abci.ResponseListSnapshots{Snapshots: []*abci.Snapshot{}}
	if app.snapshotManager == nil {
		return resp
	}

	snapshots, err := app.snapshotManager.List()
	if err != nil {
		app.logger.Error("Failed to list snapshots", "err", err)
		return resp
	}
	for _, snapshot := range snapshots {
		abciSnapshot, err := snapshot.ToABCI()
		if err != nil {
			app.logger.Error("Failed to list snapshots", "err", err)
			return resp
		}
		resp.Snapshots = append(resp.Snapshots, &abciSnapshot)
	}

	return resp
}

// LoadSnapshotChunk implements the ABCI interface. It delegates to app.snapshotManager if set.
func (app *Application) LoadSnapshotChunk(req abci.RequestLoadSnapshotChunk) abci.ResponseLoadSnapshotChunk {
	if app.snapshotManager == nil {
		return abci.ResponseLoadSnapshotChunk{}
	}
	chunk, err := app.snapshotManager.LoadChunk(req.Height, req.Format, req.Chunk)
	if err != nil {
		app.logger.Error("Failed to load snapshot chunk", "height", req.Height, "format", req.Format,
			"chunk", req.Chunk, "err")
		return abci.ResponseLoadSnapshotChunk{}
	}
	return abci.ResponseLoadSnapshotChunk{Chunk: chunk}
}

// OfferSnapshot implements the ABCI interface. It delegates to app.snapshotManager if set.
func (app *Application) OfferSnapshot(req abci.RequestOfferSnapshot) abci.ResponseOfferSnapshot {
	if req.Snapshot == nil {
		app.logger.Error("Received nil snapshot")
		return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}
	}

	snapshot, err := snapshottypes.SnapshotFromABCI(req.Snapshot)
	if err != nil {
		app.logger.Error("Failed to decode snapshot metadata", "err", err)
		return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}
	}
	err = app.snapshotManager.Restore(snapshot)
	switch {
	case err == nil:
		return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ACCEPT}

	case errors.Is(err, snapshottypes.ErrUnknownFormat):
		return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT_FORMAT}

	case errors.Is(err, snapshottypes.ErrInvalidMetadata):
		app.logger.Error("Rejecting invalid snapshot", "height", req.Snapshot.Height,
			"format", req.Snapshot.Format, "err", err)
		return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}

	default:
		app.logger.Error("Failed to restore snapshot", "height", req.Snapshot.Height,
			"format", req.Snapshot.Format, "err", err)
		// We currently don't support resetting the IAVL stores and retrying a different snapshot,
		// so we ask Tendermint to abort all snapshot restoration.
		return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ABORT}
	}
}

// ApplySnapshotChunk implements the ABCI interface. It delegates to app.snapshotManager if set.
func (app *Application) ApplySnapshotChunk(req abci.RequestApplySnapshotChunk) abci.ResponseApplySnapshotChunk {
	_, err := app.snapshotManager.RestoreChunk(req.Chunk)
	switch {
	case err == nil:
		return abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}

	case errors.Is(err, snapshottypes.ErrChunkHashMismatch):
		app.logger.Error("Chunk checksum mismatch, rejecting sender and requesting refetch",
			"chunk", req.Index, "sender", req.Sender, "err", err)
		return abci.ResponseApplySnapshotChunk{
			Result:        abci.ResponseApplySnapshotChunk_RETRY,
			RefetchChunks: []uint32{req.Index},
			RejectSenders: []string{req.Sender},
		}

	default:
		app.logger.Error("Failed to restore snapshot", "err", err)
		return abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ABORT}
	}
}
