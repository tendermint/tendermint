package kvstore

import (
	"context"
	"fmt"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
)

//-----------------------------------------

var _ types.Application = (*PersistentKVStoreApplication)(nil)

type PersistentKVStoreApplication struct {
	*Application
}

func NewPersistentKVStoreApplication(logger log.Logger, dbDir string) *PersistentKVStoreApplication {
	db, err := dbm.NewGoLevelDB("kvstore", dbDir)
	if err != nil {
		panic(fmt.Errorf("cannot open app state store: %w", err))
	}
	stateStore := &StateReaderWriter{DB: db}

	app := NewApplication(
		WithLogger(logger),
		WithStateStore(stateStore, 1),
	)

	return &PersistentKVStoreApplication{
		Application: app,
	}
}

func (app *PersistentKVStoreApplication) OfferSnapshot(_ context.Context, req *types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
	return &types.ResponseOfferSnapshot{Result: types.ResponseOfferSnapshot_ABORT}, nil
}

func (app *PersistentKVStoreApplication) ApplySnapshotChunk(_ context.Context, req *types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {
	return &types.ResponseApplySnapshotChunk{Result: types.ResponseApplySnapshotChunk_ABORT}, nil
}
