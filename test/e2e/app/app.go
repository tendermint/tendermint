package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"sync"

	db "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	types1 "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// Application is an ABCI application for use by end-to-end tests. It is a
// simple key/value store for strings, storing data in memory and persisting
// to disk as JSON, taking state sync snapshots if requested.
type Application struct {
	*kvstore.Application
	mu sync.Mutex

	logger          log.Logger
	cfg             *kvstore.Config
	restoreSnapshot *abci.Snapshot
	restoreChunks   [][]byte
}

// NewApplication creates the application.
func NewApplication(cfg kvstore.Config) (*Application, error) {
	logger, err := log.NewDefaultLogger(log.LogFormatPlain, log.LogLevelDebug)
	if err != nil {
		return nil, err
	}

	db, err := db.NewGoLevelDB("app_state", filepath.Join(cfg.Dir, "app_state"))
	if err != nil {
		return nil, err
	}

	app := Application{
		logger: logger.With("module", "abci_app"),
	}
	app.Application = kvstore.NewApplication(
		kvstore.WithLogger(logger),
		kvstore.WithConfig(cfg),
		kvstore.WithStateStore(kvstore.NewDBStateStore(db)),
	)

	return &app, nil
}

// validatorUpdates generates a validator set update.
func (app *Application) chainLockUpdate(height uint64) (*types1.CoreChainLock, error) {
	updates := app.cfg.ChainLockUpdates[fmt.Sprintf("%v", height)]
	if len(updates) == 0 {
		return nil, nil
	}

	chainLockUpdateString := app.cfg.ChainLockUpdates[fmt.Sprintf("%v", height)]
	if len(chainLockUpdateString) == 0 {
		return nil, fmt.Errorf("chainlockUpdate must be set")
	}
	chainlockUpdateHeight, err := strconv.Atoi(chainLockUpdateString)
	if err != nil {
		return nil, fmt.Errorf("invalid number chainlockUpdate value %q: %w", chainLockUpdateString, err)
	}
	chainLock := types.NewMockChainLock(uint32(chainlockUpdateHeight))
	return chainLock.ToProto(), nil

}
