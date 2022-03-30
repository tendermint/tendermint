package kvstore

import (
	"bytes"
	"encoding/base64"
	"strings"

	"github.com/gogo/protobuf/proto"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	"github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/libs/log"
)

const ValidatorSetUpdatePrefix string = "vsu:"

//-----------------------------------------

var _ types.Application = (*PersistentKVStoreApplication)(nil)

type PersistentKVStoreApplication struct {
	mtx    sync.Mutex
	app    *Application
	logger log.Logger

	valUpdatesRepo      *repository
	ValidatorSetUpdates types.ValidatorSetUpdate
}

func NewPersistentKVStoreApplication(dbDir string) *PersistentKVStoreApplication {
	const name = "kvstore"
	db, err := dbm.NewGoLevelDB(name, dbDir)
	if err != nil {
		panic(err)
	}
	return &PersistentKVStoreApplication{
		app:    &Application{state: loadState(db)},
		logger: log.NewNopLogger(),

		valUpdatesRepo: &repository{db: db},
	}
}

func (app *PersistentKVStoreApplication) Close() error {
	return app.app.state.db.Close()
}

func (app *PersistentKVStoreApplication) SetLogger(l log.Logger) {
	app.logger = l
}

func (app *PersistentKVStoreApplication) Info(req types.RequestInfo) types.ResponseInfo {
	res := app.app.Info(req)
	res.LastBlockHeight = app.app.state.Height
	res.LastBlockAppHash = app.app.state.AppHash
	return res
}

// DeliverTx will deliver a tx which is either "val:proTxHash!pubkey!power" or "key=value" or just arbitrary bytes
func (app *PersistentKVStoreApplication) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	if isValidatorSetUpdateTx(req.Tx) {
		err := app.execValidatorSetTx(req.Tx)
		if err != nil {
			return types.ResponseDeliverTx{
				Code: code.CodeTypeUnknownError,
				Log:  err.Error(),
			}
		}
		return types.ResponseDeliverTx{Code: code.CodeTypeOK}
	}
	return app.app.DeliverTx(req)
}

func (app *PersistentKVStoreApplication) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx {
	return app.app.CheckTx(req)
}

// Commit makes a commit in application's state
func (app *PersistentKVStoreApplication) Commit() types.ResponseCommit {
	return app.app.Commit()
}

// Query when path=/val and data={validator address}, returns the validator update (types.ValidatorUpdate) varint encoded.
// For any other path, returns an associated value or nil if missing.
func (app *PersistentKVStoreApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	switch reqQuery.Path {
	case "/vsu":
		vsu, err := app.valUpdatesRepo.get()
		if err != nil {
			return types.ResponseQuery{
				Code: code.CodeTypeUnknownError,
				Log:  err.Error(),
			}
		}
		data, err := encodeMsg(vsu)
		if err != nil {
			return types.ResponseQuery{
				Code: code.CodeTypeEncodingError,
				Log:  err.Error(),
			}
		}
		resQuery.Key = reqQuery.Data
		resQuery.Value = data
		return
	case "/verify-chainlock":
		resQuery.Code = 0
		return resQuery
	default:
		return app.app.Query(reqQuery)
	}
}

// InitChain saves the validators in the merkle tree
func (app *PersistentKVStoreApplication) InitChain(req types.RequestInitChain) types.ResponseInitChain {
	err := app.valUpdatesRepo.set(req.ValidatorSet)
	if err != nil {
		app.logger.Error("error updating validators", "err", err)
		return types.ResponseInitChain{}
	}
	return types.ResponseInitChain{}
}

// BeginBlock tracks the block hash and header information
func (app *PersistentKVStoreApplication) BeginBlock(req types.RequestBeginBlock) types.ResponseBeginBlock {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	// reset valset changes
	app.ValidatorSetUpdates.ValidatorUpdates = make([]types.ValidatorUpdate, 0)

	return types.ResponseBeginBlock{}
}

// EndBlock updates the validator set
func (app *PersistentKVStoreApplication) EndBlock(_ types.RequestEndBlock) types.ResponseEndBlock {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	c := proto.Clone(&app.ValidatorSetUpdates).(*types.ValidatorSetUpdate)
	return types.ResponseEndBlock{ValidatorSetUpdate: c}
}

func (app *PersistentKVStoreApplication) ListSnapshots(
	req types.RequestListSnapshots) types.ResponseListSnapshots {
	return types.ResponseListSnapshots{}
}

func (app *PersistentKVStoreApplication) LoadSnapshotChunk(
	req types.RequestLoadSnapshotChunk) types.ResponseLoadSnapshotChunk {
	return types.ResponseLoadSnapshotChunk{}
}

func (app *PersistentKVStoreApplication) OfferSnapshot(
	req types.RequestOfferSnapshot) types.ResponseOfferSnapshot {
	return types.ResponseOfferSnapshot{Result: types.ResponseOfferSnapshot_ABORT}
}

func (app *PersistentKVStoreApplication) ApplySnapshotChunk(
	req types.RequestApplySnapshotChunk) types.ResponseApplySnapshotChunk {
	return types.ResponseApplySnapshotChunk{Result: types.ResponseApplySnapshotChunk_ABORT}
}

//---------------------------------------------
// update validators

func (app *PersistentKVStoreApplication) ValidatorSet() (*types.ValidatorSetUpdate, error) {
	return app.valUpdatesRepo.get()
}

func (app *PersistentKVStoreApplication) execValidatorSetTx(tx []byte) error {
	vsu, err := UnmarshalValidatorSetUpdate(tx)
	if err != nil {
		return err
	}
	err = app.valUpdatesRepo.set(vsu)
	if err != nil {
		return err
	}
	app.ValidatorSetUpdates = *vsu
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
