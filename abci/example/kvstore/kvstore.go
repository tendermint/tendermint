package kvstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/gogo/protobuf/proto"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/version"
)

var (
	stateKey        = []byte("stateKey")
	kvPairPrefixKey = []byte("kvPairKey:")

	ProtocolVersion uint64 = 0x1
)

func prefixKey(key []byte) []byte {
	return append(kvPairPrefixKey, key...)
}

//---------------------------------------------------

var _ types.Application = (*Application)(nil)

type Application struct {
	types.BaseApplication
	mu sync.Mutex

	initialHeight int64

	lastCommittedState State
	// roundStates contains state for each round, indexed by AppHash.String()
	roundStates  map[string]State
	RetainBlocks int64 // blocks to retain after commit (via ResponseCommit.RetainHeight)
	logger       log.Logger

	finalizedAppHash    []byte
	validatorSetUpdates map[int64]types.ValidatorSetUpdate
}

func WithValidatorSetUpdates(validatorSetUpdates map[int64]types.ValidatorSetUpdate) func(app *Application) {
	return func(app *Application) {
		for height, vsu := range validatorSetUpdates {
			app.AddValidatorSetUpdate(vsu, height)
		}
	}
}

func WithLogger(logger log.Logger) func(app *Application) {
	return func(app *Application) {
		app.logger = logger
	}
}

func WithHeight(height int64) func(app *Application) {
	return func(app *Application) {
		app.lastCommittedState = &kvState{
			Height: height,
		}
	}
}

func NewApplication(opts ...func(app *Application)) *Application {
	db := dbm.NewMemDB()
	state := NewKvState(db)
	if err := state.Load(); err != nil {
		panic(fmt.Sprintf("cannot load state: %s", err))
	}

	app := &Application{
		logger:              log.NewNopLogger(),
		lastCommittedState:  state,
		roundStates:         map[string]State{},
		validatorSetUpdates: make(map[int64]types.ValidatorSetUpdate),
		initialHeight:       1,
	}

	for _, opt := range opts {
		opt(app)
	}
	return app
}

func (app *Application) InitChain(_ context.Context, req *types.RequestInitChain) (*types.ResponseInitChain, error) {
	app.mu.Lock()
	defer app.mu.Unlock()
	if req.InitialHeight != 0 {
		app.initialHeight = req.InitialHeight
	}
	if req.ValidatorSet != nil {
		app.validatorSetUpdates[req.InitialHeight] = *req.ValidatorSet
	}
	if err := app.newHeight(app.initialHeight, make([]byte, crypto.DefaultAppHashSize)); err != nil {
		panic(err)
	}
	return &types.ResponseInitChain{}, nil
}

func (app *Application) PrepareProposal(_ context.Context, req *types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	app.logger.Debug("prepare proposal", "req", req)

	roundState, txResults, err := app.handleProposal(req.Height, req.Txs)
	if err != nil {
		return &types.ResponsePrepareProposal{}, err
	}
	app.logger.Debug("end of prepare proposal", "app_hash", roundState.GetAppHash())

	return &types.ResponsePrepareProposal{
		TxRecords:             app.substPrepareTx(req.Txs, req.MaxTxBytes),
		AppHash:               roundState.GetAppHash(),
		TxResults:             txResults,
		ConsensusParamUpdates: nil,
		CoreChainLockUpdate:   nil,
		ValidatorSetUpdate:    app.getValidatorSetUpdate(req.Height),
	}, nil
}

func (app *Application) ProcessProposal(_ context.Context, req *types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {
	app.logger.Debug("process proposal", "req", req)

	roundState, txResults, err := app.handleProposal(req.Height, req.Txs)
	if err != nil {
		return &types.ResponseProcessProposal{
			Status: types.ResponseProcessProposal_REJECT,
		}, err
	}

	return &types.ResponseProcessProposal{
		Status:             types.ResponseProcessProposal_ACCEPT,
		AppHash:            roundState.GetAppHash(),
		TxResults:          txResults,
		ValidatorSetUpdate: app.getValidatorSetUpdate(req.Height),
	}, nil
}

func (app *Application) FinalizeBlock(_ context.Context, req *types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	appHash := tmbytes.HexBytes(req.AppHash)
	_, ok := app.roundStates[appHash.String()]
	if !ok {
		return &types.ResponseFinalizeBlock{}, fmt.Errorf("state with apphash %s not found", appHash)
	}
	app.finalizedAppHash = appHash

	app.logger.Debug("finalized block", "req", req)

	return &types.ResponseFinalizeBlock{}, nil
}

func (app *Application) Commit(_ context.Context) (*types.ResponseCommit, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	if len(app.finalizedAppHash) == 0 {
		return &types.ResponseCommit{}, fmt.Errorf("no uncommitted finalized block")
	}

	err := app.newHeight(app.lastCommittedState.GetHeight()+1, app.finalizedAppHash)
	if err != nil {
		return &types.ResponseCommit{}, err
	}

	resp := &types.ResponseCommit{}
	if app.RetainBlocks > 0 && app.lastCommittedState.GetHeight() >= app.RetainBlocks {
		resp.RetainHeight = app.lastCommittedState.GetHeight() - app.RetainBlocks + 1
	}

	app.logger.Debug("commit", "resp", resp)

	return resp, nil
}

func (app *Application) Info(_ context.Context, req *types.RequestInfo) (*types.ResponseInfo, error) {
	app.mu.Lock()
	defer app.mu.Unlock()
	appHash := app.lastCommittedState.GetAppHash()
	return &types.ResponseInfo{
		Data:             fmt.Sprintf("{\"appHash\":\"%s\"}", appHash.String()),
		Version:          version.ABCIVersion,
		AppVersion:       ProtocolVersion,
		LastBlockHeight:  app.lastCommittedState.GetHeight(),
		LastBlockAppHash: app.lastCommittedState.GetAppHash(),
	}, nil
}

func (*Application) CheckTx(_ context.Context, req *types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	return &types.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}, nil
}

// Query returns an associated value or nil if missing.
func (app *Application) Query(_ context.Context, reqQuery *types.RequestQuery) (*types.ResponseQuery, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	switch reqQuery.Path {
	case "/verify-chainlock":
		return &types.ResponseQuery{
			Code: 0,
		}, nil
	case "/val":
		vu, err := app.findValidatorUpdate(reqQuery.Data)
		if err != nil {
			return &types.ResponseQuery{
				Code: code.CodeTypeUnknownError,
				Log:  err.Error(),
			}, nil
		}
		value, err := encodeMsg(&vu)
		if err != nil {
			return &types.ResponseQuery{
				Code: code.CodeTypeEncodingError,
				Log:  err.Error(),
			}, nil
		}
		return &types.ResponseQuery{
			Key:   reqQuery.Data,
			Value: value,
		}, nil
	}

	if reqQuery.Prove {
		value, err := app.lastCommittedState.Get(prefixKey(reqQuery.Data))
		if err != nil {
			panic(err)
		}

		resQuery := types.ResponseQuery{
			Index:  -1,
			Key:    reqQuery.Data,
			Value:  value,
			Height: app.lastCommittedState.GetHeight(),
		}

		if value == nil {
			resQuery.Log = "does not exist"
		} else {
			resQuery.Log = "exists"
		}

		return &resQuery, nil
	}

	value, err := app.lastCommittedState.Get(prefixKey(reqQuery.Data))
	if err != nil {
		panic(err)
	}

	resQuery := types.ResponseQuery{
		Key:    reqQuery.Data,
		Value:  value,
		Height: app.lastCommittedState.GetHeight(),
	}

	if value == nil {
		resQuery.Log = "does not exist"
	} else {
		resQuery.Log = "exists"
	}

	return &resQuery, nil
}

// AddValidatorSetUpdate ...
func (app *Application) AddValidatorSetUpdate(vsu types.ValidatorSetUpdate, height int64) {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.validatorSetUpdates[height] = vsu
}

func (app *Application) Close() error {
	app.mu.Lock()
	defer app.mu.Unlock()

	app.resetRoundStates()
	return app.lastCommittedState.Close()
}

func (app *Application) newRound(height int64) (State, error) {
	if height != app.lastCommittedState.GetHeight()+1 {
		return &kvState{}, fmt.Errorf("invalid height: expected: %d, got: %d", app.lastCommittedState.GetHeight()+1, height)
	}
	roundState := &kvState{DB: dbm.NewMemDB()}
	err := app.lastCommittedState.Copy(roundState)
	roundState.Height = height
	if err != nil {
		return &kvState{}, fmt.Errorf("cannot copy current state: %w", err)
	}
	// overwrite what was set in Copy, as we are at new height
	roundState.Height = height
	return roundState, nil
}

// newHeight frees resources from previous height and starts new height.
// `height` shall be new height, and `committedRound` shall be round from previous commit
// Caller should lock the Application.
func (app *Application) newHeight(height int64, committedAppHash tmbytes.HexBytes) error {
	if height != app.lastCommittedState.GetHeight()+1 {
		return fmt.Errorf("invalid height: expected: %d, got: %d", app.lastCommittedState.GetHeight()+1, height)
	}

	// Committed round becomes new state
	// Note it can be empty (eg. on initial height), but State.Copy() should handle it
	err := app.roundStates[committedAppHash.String()].Copy(app.lastCommittedState)
	if err != nil {
		return err
	}

	app.resetRoundStates()
	if err := app.lastCommittedState.Save(); err != nil {
		return err
	}
	app.finalizedAppHash = nil

	return nil
}

func (app *Application) resetRoundStates() {
	for _, state := range app.roundStates {
		state.Close()
	}
	app.roundStates = map[string]State{}
}

func (app *Application) handleProposal(height int64, txs [][]byte) (State, []*types.ExecTxResult, error) {
	roundState, err := app.newRound(height)
	if err != nil {
		return nil, nil, err
	}

	// execute block
	txResults := make([]*types.ExecTxResult, len(txs))
	for i, tx := range txs {
		txResults[i] = app.handleTx(roundState, tx)
	}

	// Don't update AppHash at genesis height
	if roundState.GetHeight() != app.initialHeight {
		if err = roundState.UpdateAppHash(app.lastCommittedState, txs, txResults); err != nil {
			return nil, nil, fmt.Errorf("update apphash: %w", err)
		}
	}
	app.roundStates[roundState.GetAppHash().String()] = roundState

	return roundState, txResults, nil
}

//---------------------------------------------

func (app *Application) getValidatorSetUpdate(height int64) *types.ValidatorSetUpdate {
	vsu, ok := app.validatorSetUpdates[height]
	if !ok {
		var prev int64
		for h, v := range app.validatorSetUpdates {
			if h < height && prev <= h {
				vsu = v
				prev = h
			}
		}
	}
	return proto.Clone(&vsu).(*types.ValidatorSetUpdate)
}

// -----------------------------
// prepare proposal machinery

const PreparePrefix = "prepare"

func isPrepareTx(tx []byte) bool {
	return bytes.HasPrefix(tx, []byte(PreparePrefix))
}

// execPrepareTx is noop. tx data is considered as placeholder
// and is substitute at the PrepareProposal.
func (app *Application) execPrepareTx(tx []byte) *types.ExecTxResult {
	// noop
	return &types.ExecTxResult{}
}

// substPrepareTx substitutes all the transactions prefixed with 'prepare' in the
// proposal for transactions with the prefix stripped.
// It marks all of the original transactions as 'REMOVED' so that
// Tendermint will remove them from its mempool.
func (app *Application) substPrepareTx(blockData [][]byte, maxTxBytes int64) []*types.TxRecord {
	trs := make([]*types.TxRecord, 0, len(blockData))
	var removed []*types.TxRecord
	var totalBytes int64
	for _, tx := range blockData {
		txMod := tx
		action := types.TxRecord_UNMODIFIED
		if isPrepareTx(tx) {
			removed = append(removed, &types.TxRecord{
				Tx:     tx,
				Action: types.TxRecord_REMOVED,
			})
			txMod = bytes.TrimPrefix(tx, []byte(PreparePrefix))
			action = types.TxRecord_ADDED
		}
		totalBytes += int64(len(txMod))
		if totalBytes > maxTxBytes {
			break
		}
		trs = append(trs, &types.TxRecord{
			Tx:     txMod,
			Action: action,
		})
	}

	return append(trs, removed...)
}

// tx is either "val:pubkey!power" or "key=value" or just arbitrary bytes
func (app *Application) handleTx(roundState State, tx []byte) *types.ExecTxResult {
	if isPrepareTx(tx) {
		return app.execPrepareTx(tx)
	}

	var key, value string
	parts := bytes.Split(tx, []byte("="))
	if len(parts) == 2 {
		key, value = string(parts[0]), string(parts[1])
	} else {
		key, value = string(tx), string(tx)
	}

	err := roundState.Set(prefixKey([]byte(key)), []byte(value))
	if err != nil {
		panic(err)
	}

	events := []types.Event{
		{
			Type: "app",
			Attributes: []types.EventAttribute{
				{Key: "creator", Value: "Cosmoshi Netowoko", Index: true},
				{Key: "key", Value: key, Index: true},
				{Key: "index_key", Value: "index is working", Index: true},
				{Key: "noindex_key", Value: "index is working", Index: false},
			},
		},
	}

	return &types.ExecTxResult{Code: code.CodeTypeOK, Events: events}
}

func (app *Application) getActiveValidatorSetUpdates() types.ValidatorSetUpdate {
	var closestHeight int64
	for height := range app.validatorSetUpdates {
		if height > closestHeight && height <= app.lastCommittedState.GetHeight() {
			closestHeight = height
		}
	}
	return app.validatorSetUpdates[closestHeight]
}

func (app *Application) findValidatorUpdate(proTxHash crypto.ProTxHash) (types.ValidatorUpdate, error) {
	vsu := app.getActiveValidatorSetUpdates()
	for _, vu := range vsu.ValidatorUpdates {
		if proTxHash.Equal(vu.ProTxHash) {
			return vu, nil
		}
	}
	return types.ValidatorUpdate{}, errors.New("validator-update not found")
}

func encodeMsg(data proto.Message) ([]byte, error) {
	buf := bytes.NewBufferString("")
	w := protoio.NewDelimitedWriter(buf)
	_, err := w.WriteMsg(data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
