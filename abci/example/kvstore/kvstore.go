package kvstore

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/libs/log"
	cryptoproto "github.com/tendermint/tendermint/proto/tendermint/crypto"
	"github.com/tendermint/tendermint/version"
)

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

	// validator set
	ValUpdates         []types.ValidatorUpdate
	valAddrToPubKeyMap map[string]cryptoproto.PublicKey
}

func NewApplication() *Application {
	return &Application{
		logger:             log.NewNopLogger(),
		state:              loadState(dbm.NewMemDB()),
		valAddrToPubKeyMap: make(map[string]cryptoproto.PublicKey),
	}
}

func (app *Application) InitChain(_ context.Context, req *types.RequestInitChain) (*types.ResponseInitChain, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	for _, v := range req.Validators {
		r := app.updateValidator(v)
		if r.IsErr() {
			app.logger.Error("error updating validators", "r", r)
			panic("problem updating validators")
		}
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
	// if it starts with "val:", update the validator set
	// format is "val:pubkey!power"
	if isValidatorTx(tx) {
		// update validators in the merkle tree
		// and in app.ValUpdates
		return app.execValidatorTx(tx)
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
	app.ValUpdates = make([]types.ValidatorUpdate, 0)

	// Punish validators who committed equivocation.
	for _, ev := range req.Misbehavior {
		if ev.Type == types.MisbehaviorType_DUPLICATE_VOTE {
			addr := string(ev.Validator.Address)
			if pubKey, ok := app.valAddrToPubKeyMap[addr]; ok {
				app.updateValidator(types.ValidatorUpdate{
					PubKey: pubKey,
					Power:  ev.Validator.Power - 1,
				})
				app.logger.Info("Decreased val power by 1 because of the equivocation",
					"val", addr)
			} else {
				panic(fmt.Errorf("wanted to punish val %q but can't find it", addr))
			}
		}
	}

	respTxs := make([]*types.ExecTxResult, len(req.Txs))
	for i, tx := range req.Txs {
		respTxs[i] = app.handleTx(tx)
	}

	// Using a memdb - just return the big endian size of the db
	appHash := make([]byte, 8)
	binary.PutVarint(appHash, app.state.Size)
	app.state.AppHash = appHash
	app.state.Height++

	return &types.ResponseFinalizeBlock{TxResults: respTxs, ValidatorUpdates: app.ValUpdates, AppHash: appHash}, nil
}

func (*Application) CheckTx(_ context.Context, req *types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	return &types.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}, nil
}

func (app *Application) Commit(_ context.Context) (*types.ResponseCommit, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	saveState(app.state)

	resp := &types.ResponseCommit{}
	if app.RetainBlocks > 0 && app.state.Height >= app.RetainBlocks {
		resp.RetainHeight = app.state.Height - app.RetainBlocks + 1
	}
	return resp, nil
}

// Returns an associated value or nil if missing.
func (app *Application) Query(_ context.Context, reqQuery *types.RequestQuery) (*types.ResponseQuery, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	if reqQuery.Path == "/val" {
		key := []byte("val:" + string(reqQuery.Data))
		value, err := app.state.db.Get(key)
		if err != nil {
			panic(err)
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
		if len(tx) == 0 || isPrepareTx(tx) {
			return &types.ResponseProcessProposal{Status: types.ResponseProcessProposal_REJECT}, nil
		}
	}
	return &types.ResponseProcessProposal{Status: types.ResponseProcessProposal_ACCEPT}, nil
}

//---------------------------------------------
// update validators

func (app *Application) Validators() (validators []types.ValidatorUpdate) {
	app.mu.Lock()
	defer app.mu.Unlock()

	itr, err := app.state.db.Iterator(nil, nil)
	if err != nil {
		panic(err)
	}
	for ; itr.Valid(); itr.Next() {
		if isValidatorTx(itr.Key()) {
			validator := new(types.ValidatorUpdate)
			err := types.ReadMessage(bytes.NewBuffer(itr.Value()), validator)
			if err != nil {
				panic(err)
			}
			validators = append(validators, *validator)
		}
	}
	if err = itr.Error(); err != nil {
		panic(err)
	}
	return
}

func MakeValSetChangeTx(pubkey cryptoproto.PublicKey, power int64) []byte {
	pk, err := encoding.PubKeyFromProto(pubkey)
	if err != nil {
		panic(err)
	}
	pubStr := base64.StdEncoding.EncodeToString(pk.Bytes())
	return []byte(fmt.Sprintf("val:%s!%d", pubStr, power))
}

func isValidatorTx(tx []byte) bool {
	return strings.HasPrefix(string(tx), ValidatorSetChangePrefix)
}

// format is "val:pubkey!power"
// pubkey is a base64-encoded 32-byte ed25519 key
func (app *Application) execValidatorTx(tx []byte) *types.ExecTxResult {
	tx = tx[len(ValidatorSetChangePrefix):]

	//  get the pubkey and power
	pubKeyAndPower := strings.Split(string(tx), "!")
	if len(pubKeyAndPower) != 2 {
		return &types.ExecTxResult{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Expected 'pubkey!power'. Got %v", pubKeyAndPower)}
	}
	pubkeyS, powerS := pubKeyAndPower[0], pubKeyAndPower[1]

	// decode the pubkey
	pubkey, err := base64.StdEncoding.DecodeString(pubkeyS)
	if err != nil {
		return &types.ExecTxResult{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Pubkey (%s) is invalid base64", pubkeyS)}
	}

	// decode the power
	power, err := strconv.ParseInt(powerS, 10, 64)
	if err != nil {
		return &types.ExecTxResult{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Power (%s) is not an int", powerS)}
	}

	// update
	return app.updateValidator(types.UpdateValidator(pubkey, power, ""))
}

// add, update, or remove a validator
func (app *Application) updateValidator(v types.ValidatorUpdate) *types.ExecTxResult {
	pubkey, err := encoding.PubKeyFromProto(v.PubKey)
	if err != nil {
		panic(fmt.Errorf("can't decode public key: %w", err))
	}
	key := []byte("val:" + string(pubkey.Bytes()))

	if v.Power == 0 {
		// remove validator
		hasKey, err := app.state.db.Has(key)
		if err != nil {
			panic(err)
		}
		if !hasKey {
			pubStr := base64.StdEncoding.EncodeToString(pubkey.Bytes())
			return &types.ExecTxResult{
				Code: code.CodeTypeUnauthorized,
				Log:  fmt.Sprintf("Cannot remove non-existent validator %s", pubStr)}
		}
		if err = app.state.db.Delete(key); err != nil {
			panic(err)
		}
		delete(app.valAddrToPubKeyMap, string(pubkey.Address()))
	} else {
		// add or update validator
		value := bytes.NewBuffer(make([]byte, 0))
		if err := types.WriteMessage(&v, value); err != nil {
			return &types.ExecTxResult{
				Code: code.CodeTypeEncodingError,
				Log:  fmt.Sprintf("error encoding validator: %v", err)}
		}
		if err = app.state.db.Set(key, value.Bytes()); err != nil {
			panic(err)
		}
		app.valAddrToPubKeyMap[string(pubkey.Address())] = v.PubKey
	}

	// we only update the changes array if we successfully updated the tree
	app.ValUpdates = append(app.ValUpdates, v)

	return &types.ExecTxResult{Code: code.CodeTypeOK}
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
