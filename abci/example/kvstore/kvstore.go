package kvstore

import (
	"bytes"
	"context"
	"encoding/json"
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

type State struct {
	db      dbm.DB
	Height  int64            `json:"height"`
	AppHash tmbytes.HexBytes `json:"app_hash"`
}

func loadState(db dbm.DB) State {
	var state State
	state.db = db
	stateBytes, err := db.Get(stateKey)
	if err != nil {
		panic(err)
	}
	if len(stateBytes) == 0 {
		return state
	}
	err = json.Unmarshal(stateBytes, &state)
	if err != nil {
		panic(err)
	}
	return state
}

func saveState(state State) {
	stateBytes, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	err = state.db.Set(stateKey, stateBytes)
	if err != nil {
		panic(err)
	}
}

func prefixKey(key []byte) []byte {
	return append(kvPairPrefixKey, key...)
}

// Copy copies the state. It ensures copy is a valid, initialized state.
// Caller should close the state once it's not needed anymore
// newDBfunc can be provided to define DB that will be used for this copy.
func (s State) Copy(dst *State) error {
	dst.Height = s.Height
	dst.AppHash = s.AppHash.Copy()
	// apphash is required, and should never be nil,zero-length
	if len(dst.AppHash) == 0 {
		dst.AppHash = make(tmbytes.HexBytes, crypto.DefaultAppHashSize)
	}
	if err := copyDB(s.db, dst.db); err != nil {
		return fmt.Errorf("copy state db: %w", err)
	}
	return nil
}

func copyDB(src dbm.DB, dst dbm.DB) error {
	dstBatch := dst.NewBatch()
	defer dstBatch.Close()

	// cleanup dest DB first
	dstIter, err := dst.Iterator(nil, nil)
	if err != nil {
		return fmt.Errorf("cannot create dest db iterator: %w", err)
	}
	defer dstIter.Close()

	// Delete content of dst, to be sure that it will not contain any unexpected data.
	keys := make([][]byte, 0)
	for dstIter.Valid() {
		keys = append(keys, dstIter.Key())
		dstIter.Next()
	}
	for _, key := range keys {
		_ = dstBatch.Delete(key) // ignore errors
	}

	// write source to dest
	if src != nil {
		srcIter, err := src.Iterator(nil, nil)
		if err != nil {
			return fmt.Errorf("cannot copy current DB: %w", err)
		}
		defer srcIter.Close()

		for srcIter.Valid() {
			if err = dstBatch.Set(srcIter.Key(), srcIter.Value()); err != nil {
				return err
			}
			srcIter.Next()
		}

		if err = dstBatch.Write(); err != nil {
			return fmt.Errorf("cannot close dest batch: %w", err)
		}
	}

	return nil
}

func (s *State) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
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

func WithState(height int64) func(app *Application) {
	return func(app *Application) {
		app.lastCommittedState = State{
			Height: height,
		}
	}
}

func NewApplication(opts ...func(app *Application)) *Application {
	db := dbm.NewMemDB()
	app := &Application{
		logger:              log.NewNopLogger(),
		lastCommittedState:  loadState(db),
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
	app.logger.Debug("end of prepare proposal", "app_hash", roundState.AppHash)

	return &types.ResponsePrepareProposal{
		TxRecords:             app.substPrepareTx(req.Txs, req.MaxTxBytes),
		AppHash:               roundState.AppHash,
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
		AppHash:            roundState.AppHash,
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

	err := app.newHeight(app.lastCommittedState.Height+1, app.finalizedAppHash)
	if err != nil {
		return &types.ResponseCommit{}, err
	}

	resp := &types.ResponseCommit{Data: app.lastCommittedState.AppHash}
	if app.RetainBlocks > 0 && app.lastCommittedState.Height >= app.RetainBlocks {
		resp.RetainHeight = app.lastCommittedState.Height - app.RetainBlocks + 1
	}

	app.logger.Debug("commit", "resp", resp)

	return resp, nil
}

func (app *Application) Info(_ context.Context, req *types.RequestInfo) (*types.ResponseInfo, error) {
	app.mu.Lock()
	defer app.mu.Unlock()
	return &types.ResponseInfo{
		Data:             fmt.Sprintf("{\"appHash\":\"%s\"}", app.lastCommittedState.AppHash.String()),
		Version:          version.ABCIVersion,
		AppVersion:       ProtocolVersion,
		LastBlockHeight:  app.lastCommittedState.Height,
		LastBlockAppHash: app.lastCommittedState.AppHash,
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
		value, err := app.lastCommittedState.db.Get(prefixKey(reqQuery.Data))
		if err != nil {
			panic(err)
		}

		resQuery := types.ResponseQuery{
			Index:  -1,
			Key:    reqQuery.Data,
			Value:  value,
			Height: app.lastCommittedState.Height,
		}

		if value == nil {
			resQuery.Log = "does not exist"
		} else {
			resQuery.Log = "exists"
		}

		return &resQuery, nil
	}

	value, err := app.lastCommittedState.db.Get(prefixKey(reqQuery.Data))
	if err != nil {
		panic(err)
	}

	resQuery := types.ResponseQuery{
		Key:    reqQuery.Data,
		Value:  value,
		Height: app.lastCommittedState.Height,
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

// updateAppHash updates app hash for the current app state.
func (s *State) updateAppHash(lastAppHash tmbytes.HexBytes, txResults []*types.ExecTxResult) error {
	txResultsHash, err := types.TxResultsHash(txResults)
	if err != nil {
		return err
	}
	s.AppHash = crypto.Checksum(append(lastAppHash, txResultsHash...))

	return nil
}

func (app *Application) newRound(height int64) (State, error) {
	if height != app.lastCommittedState.Height+1 {
		return State{}, fmt.Errorf("invalid height: expected: %d, got: %d", app.lastCommittedState.Height+1, height)
	}
	roundState := State{db: dbm.NewMemDB()}
	err := app.lastCommittedState.Copy(&roundState)
	roundState.Height = height
	if err != nil {
		return State{}, fmt.Errorf("cannot copy current state: %w", err)
	}
	// overwrite what was set in Copy, as we are at new height
	roundState.Height = height
	return roundState, nil
}

// newHeight frees resources from previous height and starts new height.
// `height` shall be new height, and `committedRound` shall be round from previous commit
// Caller should lock the Application.
func (app *Application) newHeight(height int64, committedAppHash tmbytes.HexBytes) error {
	if height != app.lastCommittedState.Height+1 {
		return fmt.Errorf("invalid height: expected: %d, got: %d", app.lastCommittedState.Height+1, height)
	}

	// Committed round becomes new state
	// Note it can be empty (eg. on initial height), but State.Copy() should handle it
	err := app.roundStates[committedAppHash.String()].Copy(&app.lastCommittedState)
	if err != nil {
		return err
	}

	app.resetRoundStates()
	saveState(app.lastCommittedState)
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
		return State{}, nil, err
	}

	// execute block
	txResults := make([]*types.ExecTxResult, len(txs))
	for i, tx := range txs {
		txResults[i] = app.handleTx(&roundState, tx)
	}

	// Don't update AppHash at genesis height
	if roundState.Height != app.initialHeight {
		if err = roundState.updateAppHash(app.lastCommittedState.AppHash, txResults); err != nil {
			return State{}, nil, fmt.Errorf("update apphash: %w", err)
		}
	}
	app.roundStates[roundState.AppHash.String()] = roundState

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
func (app *Application) handleTx(roundState *State, tx []byte) *types.ExecTxResult {
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

	err := roundState.db.Set(prefixKey([]byte(key)), []byte(value))
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
		if height > closestHeight && height <= app.lastCommittedState.Height {
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
