package kvstore

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/code"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	types1 "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

const (
	storeKey        = "stateStoreKey"
	kvPairPrefixKey = "kvPairKey:"

	voteExtensionMaxVal int64  = 128
	voteExtensionKey    string = "extensionSum"

	ProtocolVersion uint64 = 0x1
)

func prefixKey(key []byte) []byte {
	return append([]byte(kvPairPrefixKey), key...)
}

//---------------------------------------------------

var _ abci.Application = (*Application)(nil)

type Application struct {
	abci.BaseApplication
	mu sync.Mutex

	lastCommittedState State
	// roundStates contains state for each round, indexed by AppHash.String()
	roundStates  map[string]State
	RetainBlocks int64 // blocks to retain after commit (via ResponseCommit.RetainHeight)
	logger       log.Logger

	finalizedAppHash    []byte
	validatorSetUpdates map[int64]abci.ValidatorSetUpdate

	store     Store
	processor TxProcessor

	// Genesis configuration

	cfg Config

	initialHeight         int64
	initialCoreLockHeight uint32

	// Snapshots

	snapshots       *SnapshotStore
	restoreSnapshot *abci.Snapshot
	restoreChunks   [][]byte
}

// WithValidatorSetUpdates defines initial validator set when creating Application
func WithValidatorSetUpdates(validatorSetUpdates map[int64]abci.ValidatorSetUpdate) func(app *Application) {
	return func(app *Application) {
		for height, vsu := range validatorSetUpdates {
			app.AddValidatorSetUpdate(vsu, height)
		}
	}
}

// WithLogger sets logger when creating Application
func WithLogger(logger log.Logger) func(app *Application) {
	return func(app *Application) {
		app.logger = logger
	}
}

// WithHeight sets last committed height when creating Application
func WithHeight(height int64) func(app *Application) {
	return func(app *Application) {
		state := app.lastCommittedState.(*kvState)
		state.Height = height
	}
}

// WithConfig provides Config to new Application
func WithConfig(config Config) func(app *Application) {
	return func(app *Application) {
		app.cfg = config
		if config.ValidatorUpdates != nil {
			vsu, err := config.validatorSetUpdates()
			if err != nil {
				panic(err)
			}
			WithValidatorSetUpdates(vsu)(app)
		}
		if config.InitAppInitialCoreHeight != 0 {
			app.initialCoreLockHeight = config.InitAppInitialCoreHeight
		}
	}
}

// WithTxProcessor provides custom transaction processing engine to the Application
func WithTxProcessor(txProcessor TxProcessor) func(app *Application) {
	return func(app *Application) {
		app.processor = txProcessor
	}
}

// WithStateStore provides Store to persist state every `Config.PersistInterval`` blocks
func WithStateStore(stateStore Store) func(app *Application) {
	return func(app *Application) {
		app.store = stateStore
	}
}

// NewApplication creates new Key/value store application.
// The application can be used for testing or as an example of ABCI
// implementation.
// It is possible to alter initial application confis with option funcs
func NewApplication(opts ...func(app *Application)) *Application {
	var err error

	db := dbm.NewMemDB()
	state := NewKvState(db)
	stateStore := NewDBStateStore(db)

	app := &Application{
		logger:              log.NewNopLogger(),
		lastCommittedState:  state,
		roundStates:         map[string]State{},
		validatorSetUpdates: make(map[int64]abci.ValidatorSetUpdate),
		initialHeight:       1,
		store:               stateStore,
		processor:           &txProcessor{},
	}

	for _, opt := range opts {
		opt(app)
	}

	if err := state.Load(stateStore); err != nil {
		panic(fmt.Errorf("load state: %w", err))
	}

	app.snapshots, err = NewSnapshotStore(path.Join(app.cfg.Dir, "snapshots"))
	if err != nil {
		panic(fmt.Errorf("init snapshot store: %w", err))
	}

	return app
}

// InitChain implements ABCI
func (app *Application) InitChain(_ context.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	if req.InitialHeight != 0 {
		app.initialHeight = req.InitialHeight
	}

	if req.InitialCoreHeight != 0 {
		app.initialCoreLockHeight = req.InitialCoreHeight
	}

	if len(req.AppStateBytes) > 0 {
		err := json.Unmarshal(req.AppStateBytes, &app.lastCommittedState)
		if err != nil {
			return &abci.ResponseInitChain{}, err
		}
	}

	if req.ValidatorSet != nil {
		// FIXME: should we move validatorSetUpdates to State?
		app.validatorSetUpdates[app.initialHeight] = *req.ValidatorSet
	}

	height := app.initialHeight
	if app.lastCommittedState.GetHeight() >= height {
		// We already have some committed state retrieved from req.AppStateBytes
		height = app.lastCommittedState.GetHeight() + 1
	}

	if err := app.newHeight(height, make([]byte, crypto.DefaultAppHashSize)); err != nil {
		return &abci.ResponseInitChain{}, err
	}

	resp := &abci.ResponseInitChain{
		AppHash: app.lastCommittedState.GetAppHash(),
		ConsensusParams: &types1.ConsensusParams{
			Version: &types1.VersionParams{
				AppVersion: ProtocolVersion,
			},
		},
		ValidatorSetUpdate: app.validatorSetUpdates[app.initialHeight],
		InitialCoreHeight:  app.initialCoreLockHeight,
		// TODO Implement core chainlock updates logic
		NextCoreChainLockUpdate: nil,
	}

	app.logger.Debug("InitChain", "req", req, "resp", resp)
	return resp, nil
}

// PrepareProposal implements ABCI
func (app *Application) PrepareProposal(_ context.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	txRecords, err := app.processor.PrepareTxs(*req)
	if err != nil {
		return &abci.ResponsePrepareProposal{}, err
	}

	roundState, txResults, err := app.executeProposal(req.Height, txRecords)
	if err != nil {
		return &abci.ResponsePrepareProposal{}, err
	}

	resp := &abci.ResponsePrepareProposal{
		TxRecords:             txRecords,
		AppHash:               roundState.GetAppHash(),
		TxResults:             txResults,
		ConsensusParamUpdates: nil, // TODO: implement
		CoreChainLockUpdate:   nil, // TODO: implement
		ValidatorSetUpdate:    app.getValidatorSetUpdate(req.Height),
	}

	if app.cfg.PrepareProposalDelayMS != 0 {
		time.Sleep(time.Duration(app.cfg.PrepareProposalDelayMS) * time.Millisecond)
	}

	app.logger.Debug("PrepareProposal", "app_hash", roundState.GetAppHash(), "req", req, "resp", resp)
	return resp, nil
}

func (app *Application) ProcessProposal(_ context.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	roundState, txResults, err := app.executeProposal(req.Height, txs2TxRecords(req.Txs))
	if err != nil {
		return &abci.ResponseProcessProposal{
			Status: abci.ResponseProcessProposal_REJECT,
		}, err
	}

	resp := &abci.ResponseProcessProposal{
		Status:             abci.ResponseProcessProposal_ACCEPT,
		AppHash:            roundState.GetAppHash(),
		TxResults:          txResults,
		ValidatorSetUpdate: app.getValidatorSetUpdate(req.Height),
	}

	if app.cfg.ProcessProposalDelayMS != 0 {
		time.Sleep(time.Duration(app.cfg.ProcessProposalDelayMS) * time.Millisecond)
	}

	app.logger.Debug("ProcessProposal", "req", req, "resp", resp)
	return resp, nil
}

// FinalizeBlock implements ABCI
func (app *Application) FinalizeBlock(_ context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	appHash := tmbytes.HexBytes(req.AppHash)
	roundState, ok := app.roundStates[appHash.String()]
	if !ok {
		return &abci.ResponseFinalizeBlock{}, fmt.Errorf("state with apphash %s not found", appHash)
	}
	if roundState.GetHeight() != req.Height {
		return &abci.ResponseFinalizeBlock{},
			fmt.Errorf("height mismatch: expected %d, got %d", roundState.GetHeight(), req.Height)
	}
	app.finalizedAppHash = appHash

	events := []abci.Event{app.eventValUpdate(req.Height)}
	resp := &abci.ResponseFinalizeBlock{
		Events: events,
	}
	if app.RetainBlocks > 0 && app.lastCommittedState.GetHeight() >= app.RetainBlocks {
		resp.RetainHeight = app.lastCommittedState.GetHeight() - app.RetainBlocks + 1
	}

	if app.cfg.FinalizeBlockDelayMS != 0 {
		time.Sleep(time.Duration(app.cfg.FinalizeBlockDelayMS) * time.Millisecond)
	}

	app.logger.Debug("FinalizeBlock", "req", req, "resp", resp)
	return resp, nil
}

// eventValUpdate generates an event that contains info about current validator set
func (app *Application) eventValUpdate(height int64) abci.Event {
	vu := app.getValidatorSetUpdate(height)
	event := abci.Event{
		Type: "val_updates",
		Attributes: []abci.EventAttribute{
			{
				Key:   "size",
				Value: strconv.Itoa(len(vu.ValidatorUpdates)),
			},
			{
				Key:   "height",
				Value: strconv.Itoa(int(height)),
			},
		},
	}

	return event
}

// Commit implements ABCI; DEPRECATED
func (app *Application) Commit(_ context.Context) (*abci.ResponseCommit, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	if len(app.finalizedAppHash) == 0 {
		return &abci.ResponseCommit{}, fmt.Errorf("no uncommitted finalized block")
	}

	err := app.newHeight(app.lastCommittedState.GetHeight()+1, app.finalizedAppHash)
	if err != nil {
		return &abci.ResponseCommit{}, err
	}

	if err := app.createSnapshot(); err != nil {
		return &abci.ResponseCommit{}, fmt.Errorf("create snapshot: %w", err)
	}

	resp := &abci.ResponseCommit{}
	if app.RetainBlocks > 0 && app.lastCommittedState.GetHeight() >= app.RetainBlocks {
		resp.RetainHeight = app.lastCommittedState.GetHeight() - app.RetainBlocks + 1
	}

	app.logger.Debug("commit", "resp", resp)
	return resp, nil
}

// ListSnapshots implements ABCI.
func (app *Application) ListSnapshots(_ context.Context, req *abci.RequestListSnapshots) (*abci.ResponseListSnapshots, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	snapshots, err := app.snapshots.List()
	if err != nil {
		return &abci.ResponseListSnapshots{}, err
	}
	resp := abci.ResponseListSnapshots{Snapshots: snapshots}

	app.logger.Debug("ListSnapshots", "req", req, "resp", resp)
	return &resp, nil
}

// LoadSnapshotChunk implements ABCI.
func (app *Application) LoadSnapshotChunk(_ context.Context, req *abci.RequestLoadSnapshotChunk) (*abci.ResponseLoadSnapshotChunk, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	chunk, err := app.snapshots.LoadChunk(req.Height, req.Format, req.Chunk)
	if err != nil {
		panic(err)
	}
	resp := &abci.ResponseLoadSnapshotChunk{Chunk: chunk}

	app.logger.Debug("LoadSnapshotChunk", "resp", resp)
	return resp, nil
}

// OfferSnapshot implements ABCI.
func (app *Application) OfferSnapshot(_ context.Context, req *abci.RequestOfferSnapshot) (*abci.ResponseOfferSnapshot, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.restoreSnapshot != nil {
		panic("A snapshot is already being restored")
	}
	app.restoreSnapshot = req.Snapshot
	app.restoreChunks = [][]byte{}
	resp := &abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ACCEPT}

	app.logger.Debug("OfferSnapshot", "req", req, "resp", resp)
	return resp, nil
}

// ApplySnapshotChunk implements ABCI.
func (app *Application) ApplySnapshotChunk(_ context.Context, req *abci.RequestApplySnapshotChunk) (*abci.ResponseApplySnapshotChunk, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.restoreSnapshot == nil {
		panic("No restore in progress")
	}
	app.restoreChunks = append(app.restoreChunks, req.Chunk)
	if len(app.restoreChunks) == int(app.restoreSnapshot.Chunks) {
		bz := []byte{}
		for _, chunk := range app.restoreChunks {
			bz = append(bz, chunk...)
		}
		if err := json.Unmarshal(bz, &app.lastCommittedState); err != nil {
			panic(err)
		}

		app.restoreSnapshot = nil
		app.restoreChunks = nil
	}

	resp := &abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}

	app.logger.Debug("ApplySnapshotChunk", "resp", resp)
	return resp, nil
}

func (app *Application) createSnapshot() error {
	height := app.lastCommittedState.GetHeight()
	if app.cfg.SnapshotInterval > 0 && uint64(height)%app.cfg.SnapshotInterval == 0 {
		if _, err := app.snapshots.Create(app.lastCommittedState); err != nil {
			return fmt.Errorf("create snapshot: %w", err)
		}
		app.logger.Info("created state sync snapshot", "height", height)
	}

	if err := app.snapshots.Prune(maxSnapshotCount); err != nil {
		return fmt.Errorf("prune snapshots: %w", err)
	}

	return nil
}

// ExtendVote will produce vote extensions in the form of random numbers to
// demonstrate vote extension nondeterminism.
//
// In the next block, if there are any vote extensions from the previous block,
// a new transaction will be proposed that updates a special value in the
// key/value store ("extensionSum") with the sum of all of the numbers collected
// from the vote extensions.
func (app *Application) ExtendVote(_ context.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	// We ignore any requests for vote extensions that don't match our expected
	// next height.
	lastHeight := app.lastCommittedState.GetHeight()
	if lastHeight == 0 {
		lastHeight = app.initialHeight
	}
	if req.Height != lastHeight+1 {
		app.logger.Error(
			"got unexpected height in ExtendVote request",
			"expectedHeight", lastHeight+1,
			"requestHeight", req.Height,
		)
		return &abci.ResponseExtendVote{}, nil
	}
	ext := make([]byte, binary.MaxVarintLen64)
	// We don't care that these values are generated by a weak random number
	// generator. It's just for test purposes.
	// nolint:gosec // G404: Use of weak random number generator
	num := rand.Int63n(voteExtensionMaxVal)
	extLen := binary.PutVarint(ext, num)
	app.logger.Info("generated vote extension",
		"num", num,
		"ext", fmt.Sprintf("%x", ext[:extLen]),
		"state.Height", lastHeight,
	)
	return &abci.ResponseExtendVote{
		VoteExtensions: []*abci.ExtendVoteExtension{
			{
				Type:      types1.VoteExtensionType_DEFAULT,
				Extension: ext[:extLen],
			},
			{
				Type:      types1.VoteExtensionType_THRESHOLD_RECOVER,
				Extension: []byte(fmt.Sprintf("threshold-%d", lastHeight+1)),
			},
		},
	}, nil
}

// VerifyVoteExtension simply validates vote extensions from other validators
// without doing anything about them. In this case, it just makes sure that the
// vote extension is a well-formed integer value.
func (app *Application) VerifyVoteExtension(_ context.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
	// We allow vote extensions to be optional
	if len(req.VoteExtensions) == 0 {
		return &abci.ResponseVerifyVoteExtension{
			Status: abci.ResponseVerifyVoteExtension_ACCEPT,
		}, nil
	}
	lastHeight := app.lastCommittedState.GetHeight()
	if req.Height != lastHeight+1 {
		app.logger.Error(
			"got unexpected height in VerifyVoteExtension request",
			"expectedHeight", lastHeight+1,
			"requestHeight", req.Height,
		)
		return &abci.ResponseVerifyVoteExtension{
			Status: abci.ResponseVerifyVoteExtension_REJECT,
		}, nil
	}

	nums := make([]int64, 0, len(req.VoteExtensions))
	for _, ext := range req.VoteExtensions {
		num, err := parseVoteExtension(ext.Extension)
		if err != nil {
			app.logger.Error("failed to verify vote extension", "req", req, "err", err)
			return &abci.ResponseVerifyVoteExtension{
				Status: abci.ResponseVerifyVoteExtension_REJECT,
			}, nil
		}
		nums = append(nums, num)
	}

	if app.cfg.VoteExtensionDelayMS != 0 {
		time.Sleep(time.Duration(app.cfg.VoteExtensionDelayMS) * time.Millisecond)
	}

	app.logger.Info("verified vote extension value", "req", req, "nums", nums)
	return &abci.ResponseVerifyVoteExtension{
		Status: abci.ResponseVerifyVoteExtension_ACCEPT,
	}, nil
}

// parseVoteExtension attempts to parse the given extension data into a positive
// integer value.
func parseVoteExtension(ext []byte) (int64, error) {
	num, errVal := binary.Varint(ext)
	if errVal == 0 {
		return 0, errors.New("vote extension is too small to parse")
	}
	if errVal < 0 {
		return 0, errors.New("vote extension value is too large")
	}
	if num >= voteExtensionMaxVal {
		return 0, fmt.Errorf("vote extension value must be smaller than %d (was %d)", voteExtensionMaxVal, num)
	}
	return num, nil
}

// Info implements ABCI
func (app *Application) Info(_ context.Context, req *abci.RequestInfo) (*abci.ResponseInfo, error) {
	app.mu.Lock()
	defer app.mu.Unlock()
	appHash := app.lastCommittedState.GetAppHash()
	return &abci.ResponseInfo{
		Data:             fmt.Sprintf("{\"appHash\":\"%s\"}", appHash.String()),
		Version:          version.ABCIVersion,
		AppVersion:       ProtocolVersion,
		LastBlockHeight:  app.lastCommittedState.GetHeight(),
		LastBlockAppHash: app.lastCommittedState.GetAppHash(),
	}, nil
}

// CheckTX implements ABCI
func (app *Application) CheckTx(_ context.Context, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	resp, err := app.processor.VerifyTx(req.Tx, req.Type)
	if app.cfg.CheckTxDelayMS != 0 {
		time.Sleep(time.Duration(app.cfg.CheckTxDelayMS) * time.Millisecond)
	}

	return &resp, err
}

// Query returns an associated value or nil if missing.
func (app *Application) Query(_ context.Context, reqQuery *abci.RequestQuery) (*abci.ResponseQuery, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	switch reqQuery.Path {
	case "/verify-chainlock":
		return &abci.ResponseQuery{
			Code: 0,
		}, nil
	case "/val":
		vu, err := app.findValidatorUpdate(reqQuery.Data)
		if err != nil {
			return &abci.ResponseQuery{
				Code: code.CodeTypeUnknownError,
				Log:  err.Error(),
			}, nil
		}
		value, err := encodeMsg(&vu)
		if err != nil {
			return &abci.ResponseQuery{
				Code: code.CodeTypeEncodingError,
				Log:  err.Error(),
			}, nil
		}
		return &abci.ResponseQuery{
			Key:   reqQuery.Data,
			Value: value,
		}, nil
	}

	if reqQuery.Prove {
		value, err := app.lastCommittedState.Get(prefixKey(reqQuery.Data))
		if err != nil {
			panic(err)
		}

		resQuery := abci.ResponseQuery{
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

	resQuery := abci.ResponseQuery{
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

// AddValidatorSetUpdate schedules new valiudator set update at some height
func (app *Application) AddValidatorSetUpdate(vsu abci.ValidatorSetUpdate, height int64) {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.validatorSetUpdates[height] = vsu
}

// Close closes the app gracefully
func (app *Application) Close() error {
	app.mu.Lock()
	defer app.mu.Unlock()

	app.resetRoundStates()
	app.lastCommittedState.Close()
	app.store.Close()

	return nil
}

// newHeight frees resources from previous height and starts new height.
// `height` shall be new height, and `committedRound` shall be round from previous commit
// Caller should lock the Application.
func (app *Application) newHeight(height int64, committedAppHash tmbytes.HexBytes) error {
	if height != app.lastCommittedState.GetHeight()+1 {
		return fmt.Errorf("invalid height: expected: %d, got: %d", app.lastCommittedState.GetHeight()+1, height)
	}

	// Committed round becomes new state
	committedState := app.roundStates[committedAppHash.String()]
	if committedState == nil {
		committedState = NewKvState(dbm.NewMemDB())
	}
	err := committedState.Copy(app.lastCommittedState)
	if err != nil {
		return err
	}

	app.resetRoundStates()
	if err := app.persist(); err != nil {
		return err
	}
	app.finalizedAppHash = nil

	return nil
}

// resetRoundStates closes and cleans up uncommitted round states
func (app *Application) resetRoundStates() {
	for _, state := range app.roundStates {
		state.Close()
	}
	app.roundStates = map[string]State{}
}

// executeProposal executes transactions and creates new candidate state
func (app *Application) executeProposal(height int64, txs []*abci.TxRecord) (State, []*abci.ExecTxResult, error) {
	roundState, err := app.lastCommittedState.NextHeightState(dbm.NewMemDB())
	if err != nil {
		return nil, nil, err
	}
	// execute block
	txResults := make([]*abci.ExecTxResult, 0, len(txs))
	for _, tx := range txs {
		if tx.Action == abci.TxRecord_REMOVED {
			continue // we don't execute removed tx records
		}
		result, err := app.processor.ExecTx(tx.Tx, roundState)
		if err != nil && result.Code == 0 {
			result = abci.ExecTxResult{Code: code.CodeTypeUnknownError, Log: err.Error()}
		}
		txResults = append(txResults, &result)
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
// getValidatorSetUpdate returns validator update at some `height`` that will be applied at `height+1`.
func (app *Application) getValidatorSetUpdate(height int64) *abci.ValidatorSetUpdate {
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
	return proto.Clone(&vsu).(*abci.ValidatorSetUpdate)
}

// -----------------------------
// validator set updates logic

func (app *Application) getActiveValidatorSetUpdates() abci.ValidatorSetUpdate {
	var closestHeight int64
	for height := range app.validatorSetUpdates {
		if height > closestHeight && height <= app.lastCommittedState.GetHeight() {
			closestHeight = height
		}
	}
	return app.validatorSetUpdates[closestHeight]
}

func (app *Application) findValidatorUpdate(proTxHash crypto.ProTxHash) (abci.ValidatorUpdate, error) {
	vsu := app.getActiveValidatorSetUpdates()
	for _, vu := range vsu.ValidatorUpdates {
		if proTxHash.Equal(vu.ProTxHash) {
			return vu, nil
		}
	}
	return abci.ValidatorUpdate{}, errors.New("validator-update not found")
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

// persist persists application state according to the config
func (app *Application) persist() error {
	if app.cfg.PersistInterval > 0 && app.lastCommittedState.GetHeight()%int64(app.cfg.PersistInterval) == 0 {
		return app.lastCommittedState.Save(app.store)
	}
	return nil
}
