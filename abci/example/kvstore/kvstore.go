package kvstore

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/gogo/protobuf/proto"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/version"
)

const ValidatorSetUpdatePrefix string = "vsu:"

var (
	stateKey        = []byte("stateKey")
	kvPairPrefixKey = []byte("kvPairKey:")

	ProtocolVersion uint64 = 0x1
)

type State struct {
	db      dbm.DB
	Size    int64  `json:"size"`
	Height  int64  `json:"height"`
	AppHash []byte `json:"app_hash"`
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

//---------------------------------------------------

var _ types.Application = (*Application)(nil)

type Application struct {
	types.BaseApplication
	mu           sync.Mutex
	state        State
	RetainBlocks int64 // blocks to retain after commit (via ResponseCommit.RetainHeight)
	logger       log.Logger

	// validator set update
	valUpdatesRepo *repository
	valSetUpdate   types.ValidatorSetUpdate
	valsIndex      map[string]*types.ValidatorUpdate
}

func NewApplication() *Application {
	db := dbm.NewMemDB()
	return &Application{
		logger:         log.NewNopLogger(),
		state:          loadState(db),
		valsIndex:      make(map[string]*types.ValidatorUpdate),
		valUpdatesRepo: &repository{db},
	}
}

func (app *Application) setValSetUpdate(valSetUpdate *types.ValidatorSetUpdate) error {
	err := app.valUpdatesRepo.set(valSetUpdate)
	if err != nil {
		return err
	}
	app.valsIndex = make(map[string]*types.ValidatorUpdate)
	for i, v := range valSetUpdate.ValidatorUpdates {
		app.valsIndex[crypto.ProTxHash(v.ProTxHash).String()] = &valSetUpdate.ValidatorUpdates[i]
	}
	return nil
}

func (app *Application) InitChain(_ context.Context, req *types.RequestInitChain) (*types.ResponseInitChain, error) {
	app.mu.Lock()
	defer app.mu.Unlock()
	err := app.setValSetUpdate(req.ValidatorSet)
	if err != nil {
		return nil, err
	}
	return &types.ResponseInitChain{}, nil
}

func (app *Application) Info(_ context.Context, req *types.RequestInfo) (*types.ResponseInfo, error) {
	app.mu.Lock()
	defer app.mu.Unlock()
	return &types.ResponseInfo{
		Data:             fmt.Sprintf("{\"size\":%v}", app.state.Size),
		Version:          version.ABCIVersion,
		AppVersion:       ProtocolVersion,
		LastBlockHeight:  app.state.Height,
		LastBlockAppHash: app.state.AppHash,
	}, nil
}

// tx is either "val:pubkey!power" or "key=value" or just arbitrary bytes
func (app *Application) handleTx(tx []byte) *types.ExecTxResult {
	if isValidatorSetUpdateTx(tx) {
		err := app.execValidatorSetTx(tx)
		if err != nil {
			return &types.ExecTxResult{
				Code: code.CodeTypeUnknownError,
				Log:  err.Error(),
			}
		}
		return &types.ExecTxResult{Code: code.CodeTypeOK}
	}

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

	err := app.state.db.Set(prefixKey([]byte(key)), []byte(value))
	if err != nil {
		panic(err)
	}
	app.state.Size++

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

func (app *Application) Close() error {
	app.mu.Lock()
	defer app.mu.Unlock()

	return app.state.db.Close()
}

func (app *Application) FinalizeBlock(_ context.Context, req *types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	// reset valset changes
	app.valSetUpdate = types.ValidatorSetUpdate{}
	app.valSetUpdate.ValidatorUpdates = make([]types.ValidatorUpdate, 0)
	app.valsIndex = make(map[string]*types.ValidatorUpdate)

	// Punish validators who committed equivocation.
	//for _, ev := range req.ByzantineValidators {
	//	// TODO it seems this code is not needed to keep here
	//	if ev.Type == types.MisbehaviorType_DUPLICATE_VOTE {
	//		proTxHash := crypto.ProTxHash(ev.Validator.ProTxHash)
	//		v, ok := app.valsIndex[proTxHash.String()]
	//		if !ok {
	//			return nil, fmt.Errorf("wanted to punish val %q but can't find it", proTxHash.ShortString())
	//		}
	//		v.Power = ev.Validator.Power - 1
	//	}
	//}

	respTxs := make([]*types.ExecTxResult, len(req.Txs))
	for i, tx := range req.Txs {
		respTxs[i] = app.handleTx(tx)
	}

	return &types.ResponseFinalizeBlock{
		TxResults:          respTxs,
		ValidatorSetUpdate: proto.Clone(&app.valSetUpdate).(*types.ValidatorSetUpdate),
	}, nil
}

func (*Application) CheckTx(_ context.Context, req *types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	return &types.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}, nil
}

func (app *Application) Commit(_ context.Context) (*types.ResponseCommit, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	// Using a memdb - just return the big endian size of the db
	appHash := make([]byte, 32)
	binary.PutVarint(appHash, app.state.Size)
	app.state.AppHash = appHash
	app.state.Height++
	saveState(app.state)

	resp := &types.ResponseCommit{Data: appHash}
	if app.RetainBlocks > 0 && app.state.Height >= app.RetainBlocks {
		resp.RetainHeight = app.state.Height - app.RetainBlocks + 1
	}
	return resp, nil
}

// Query returns an associated value or nil if missing.
func (app *Application) Query(_ context.Context, reqQuery *types.RequestQuery) (*types.ResponseQuery, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	switch reqQuery.Path {
	case "/vsu":
		vsu, err := app.valUpdatesRepo.get()
		if err != nil {
			return &types.ResponseQuery{
				Code: code.CodeTypeUnknownError,
				Log:  err.Error(),
			}, nil
		}
		data, err := encodeMsg(vsu)
		if err != nil {
			return &types.ResponseQuery{
				Code: code.CodeTypeEncodingError,
				Log:  err.Error(),
			}, nil
		}
		return &types.ResponseQuery{
			Key:   reqQuery.Data,
			Value: data,
		}, nil
	case "/verify-chainlock":
		return &types.ResponseQuery{
			Code: 0,
		}, nil
	case "/val":
		vu, err := app.valUpdatesRepo.findBy(reqQuery.Data)
		if err != nil {
			return &types.ResponseQuery{
				Code: code.CodeTypeUnknownError,
				Log:  err.Error(),
			}, nil
		}
		value, err := encodeMsg(vu)
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
		value, err := app.state.db.Get(prefixKey(reqQuery.Data))
		if err != nil {
			panic(err)
		}

		resQuery := types.ResponseQuery{
			Index:  -1,
			Key:    reqQuery.Data,
			Value:  value,
			Height: app.state.Height,
		}

		if value == nil {
			resQuery.Log = "does not exist"
		} else {
			resQuery.Log = "exists"
		}

		return &resQuery, nil
	}

	value, err := app.state.db.Get(prefixKey(reqQuery.Data))
	if err != nil {
		panic(err)
	}

	resQuery := types.ResponseQuery{
		Key:    reqQuery.Data,
		Value:  value,
		Height: app.state.Height,
	}

	if value == nil {
		resQuery.Log = "does not exist"
	} else {
		resQuery.Log = "exists"
	}

	return &resQuery, nil
}

func (app *Application) PrepareProposal(_ context.Context, req *types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	return &types.ResponsePrepareProposal{
		TxRecords: app.substPrepareTx(req.Txs, req.MaxTxBytes),
	}, nil
}

func (*Application) ProcessProposal(_ context.Context, req *types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {
	for _, tx := range req.Txs {
		if len(tx) == 0 {
			return &types.ResponseProcessProposal{Status: types.ResponseProcessProposal_REJECT}, nil
		}
	}
	return &types.ResponseProcessProposal{Status: types.ResponseProcessProposal_ACCEPT}, nil
}

//---------------------------------------------
// update validators

func (app *Application) ValidatorSet() (*types.ValidatorSetUpdate, error) {
	return app.valUpdatesRepo.get()
}

func (app *Application) execValidatorSetTx(tx []byte) error {
	vsu, err := UnmarshalValidatorSetUpdate(tx)
	if err != nil {
		return err
	}
	err = app.valUpdatesRepo.set(vsu)
	if err != nil {
		return err
	}
	app.valSetUpdate = *vsu
	app.valsIndex = make(map[string]*types.ValidatorUpdate)
	for i, v := range vsu.ValidatorUpdates {
		app.valsIndex[v.String()] = &vsu.ValidatorUpdates[i]
	}
	return nil
}

// MarshalValidatorSetUpdate encodes validator-set-update into protobuf, encode into base64 and add "vsu:" prefix
func MarshalValidatorSetUpdate(vsu *types.ValidatorSetUpdate) ([]byte, error) {
	pbData, err := proto.Marshal(vsu)
	if err != nil {
		return nil, err
	}
	return []byte(ValidatorSetUpdatePrefix + base64.StdEncoding.EncodeToString(pbData)), nil
}

// UnmarshalValidatorSetUpdate removes "vsu:" prefix and unmarshal a string into validator-set-update
func UnmarshalValidatorSetUpdate(data []byte) (*types.ValidatorSetUpdate, error) {
	l := len(ValidatorSetUpdatePrefix)
	data, err := base64.StdEncoding.DecodeString(string(data[l:]))
	if err != nil {
		return nil, err
	}
	vsu := new(types.ValidatorSetUpdate)
	err = proto.Unmarshal(data, vsu)
	return vsu, err
}

type repository struct {
	db dbm.DB
}

func (r *repository) set(vsu *types.ValidatorSetUpdate) error {
	data, err := proto.Marshal(vsu)
	if err != nil {
		return err
	}
	return r.db.Set([]byte(ValidatorSetUpdatePrefix), data)
}

func (r *repository) get() (*types.ValidatorSetUpdate, error) {
	data, err := r.db.Get([]byte(ValidatorSetUpdatePrefix))
	if err != nil {
		return nil, err
	}
	vsu := new(types.ValidatorSetUpdate)
	err = proto.Unmarshal(data, vsu)
	if err != nil {
		return nil, err
	}
	return vsu, nil
}

func (r *repository) findBy(proTxHash crypto.ProTxHash) (*types.ValidatorUpdate, error) {
	vsu, err := r.get()
	if err != nil {
		return nil, err
	}
	for _, vu := range vsu.ValidatorUpdates {
		if bytes.Equal(vu.ProTxHash, proTxHash) {
			return &vu, nil
		}
	}
	return nil, err
}

func isValidatorSetUpdateTx(tx []byte) bool {
	return strings.HasPrefix(string(tx), ValidatorSetUpdatePrefix)
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
