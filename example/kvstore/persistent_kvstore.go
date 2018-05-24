package kvstore

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/tendermint/abci/example/code"
	"github.com/tendermint/abci/types"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"
)

const (
	ValidatorSetChangePrefix string = "val:"
)

//-----------------------------------------

var _ types.Application = (*PersistentKVStoreApplication)(nil)

type PersistentKVStoreApplication struct {
	app *KVStoreApplication

	// validator set
	ValUpdates []types.Validator

	logger log.Logger
}

func NewPersistentKVStoreApplication(dbDir string) *PersistentKVStoreApplication {
	name := "kvstore"
	db, err := dbm.NewGoLevelDB(name, dbDir)
	if err != nil {
		panic(err)
	}

	state := loadState(db)

	return &PersistentKVStoreApplication{
		app:    &KVStoreApplication{state: state},
		logger: log.NewNopLogger(),
	}
}

func (app *PersistentKVStoreApplication) SetLogger(l log.Logger) {
	app.logger = l
}

func (app *PersistentKVStoreApplication) Info(req types.ParamsInfo) types.ResultInfo {
	res := app.app.Info(req)
	res.LastBlockHeight = app.app.state.Height
	res.LastBlockAppHash = app.app.state.AppHash
	return res
}

func (app *PersistentKVStoreApplication) SetOption(req types.ParamsSetOption) types.ResultSetOption {
	return app.app.SetOption(req)
}

// tx is either "val:pubkey/power" or "key=value" or just arbitrary bytes
func (app *PersistentKVStoreApplication) DeliverTx(tx []byte) types.ResultDeliverTx {
	// if it starts with "val:", update the validator set
	// format is "val:pubkey/power"
	if isValidatorTx(tx) {
		// update validators in the merkle tree
		// and in app.ValUpdates
		return app.execValidatorTx(tx)
	}

	// otherwise, update the key-value store
	return app.app.DeliverTx(tx)
}

func (app *PersistentKVStoreApplication) CheckTx(tx []byte) types.ResultCheckTx {
	return app.app.CheckTx(tx)
}

// Commit will panic if InitChain was not called
func (app *PersistentKVStoreApplication) Commit() types.ResultCommit {
	return app.app.Commit()
}

func (app *PersistentKVStoreApplication) Query(reqQuery types.ParamsQuery) types.ResultQuery {
	return app.app.Query(reqQuery)
}

// Save the validators in the merkle tree
func (app *PersistentKVStoreApplication) InitChain(req types.ParamsInitChain) types.ResultInitChain {
	for _, v := range req.Validators {
		r := app.updateValidator(v)
		if r.IsErr() {
			app.logger.Error("Error updating validators", "r", r)
		}
	}
	return types.ResultInitChain{}
}

// Track the block hash and header information
func (app *PersistentKVStoreApplication) BeginBlock(req types.ParamsBeginBlock) types.ResultBeginBlock {
	// reset valset changes
	app.ValUpdates = make([]types.Validator, 0)
	return types.ResultBeginBlock{}
}

// Update the validator set
func (app *PersistentKVStoreApplication) EndBlock(req types.ParamsEndBlock) types.ResultEndBlock {
	return types.ResultEndBlock{ValidatorUpdates: app.ValUpdates}
}

//---------------------------------------------
// update validators

func (app *PersistentKVStoreApplication) Validators() (validators []types.Validator) {
	itr := app.app.state.db.Iterator(nil, nil)
	for ; itr.Valid(); itr.Next() {
		if isValidatorTx(itr.Key()) {
			validator := new(types.Validator)
			err := types.ReadMessage(bytes.NewBuffer(itr.Value()), validator)
			if err != nil {
				panic(err)
			}
			validators = append(validators, *validator)
		}
	}
	return
}

func MakeValSetChangeTx(pubkey types.PubKey, power int64) []byte {
	return []byte(cmn.Fmt("val:%X/%d", pubkey.Data, power))
}

func isValidatorTx(tx []byte) bool {
	return strings.HasPrefix(string(tx), ValidatorSetChangePrefix)
}

// format is "val:pubkey/power"
// pubkey is raw 32-byte ed25519 key
func (app *PersistentKVStoreApplication) execValidatorTx(tx []byte) types.ResultDeliverTx {
	tx = tx[len(ValidatorSetChangePrefix):]

	//get the pubkey and power
	pubKeyAndPower := strings.Split(string(tx), "/")
	if len(pubKeyAndPower) != 2 {
		return types.ResultDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Expected 'pubkey/power'. Got %v", pubKeyAndPower)}
	}
	pubkeyS, powerS := pubKeyAndPower[0], pubKeyAndPower[1]

	// decode the pubkey
	pubkey, err := hex.DecodeString(pubkeyS)
	if err != nil {
		return types.ResultDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Pubkey (%s) is invalid hex", pubkeyS)}
	}

	// decode the power
	power, err := strconv.ParseInt(powerS, 10, 64)
	if err != nil {
		return types.ResultDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Power (%s) is not an int", powerS)}
	}

	// update
	return app.updateValidator(types.Ed25519Validator(pubkey, int64(power)))
}

// add, update, or remove a validator
func (app *PersistentKVStoreApplication) updateValidator(v types.Validator) types.ResultDeliverTx {
	key := []byte("val:" + string(v.PubKey.Data))
	if v.Power == 0 {
		// remove validator
		if !app.app.state.db.Has(key) {
			return types.ResultDeliverTx{
				Code: code.CodeTypeUnauthorized,
				Log:  fmt.Sprintf("Cannot remove non-existent validator %X", key)}
		}
		app.app.state.db.Delete(key)
	} else {
		// add or update validator
		value := bytes.NewBuffer(make([]byte, 0))
		if err := types.WriteMessage(&v, value); err != nil {
			return types.ResultDeliverTx{
				Code: code.CodeTypeEncodingError,
				Log:  fmt.Sprintf("Error encoding validator: %v", err)}
		}
		app.app.state.db.Set(key, value.Bytes())
	}

	// we only update the changes array if we successfully updated the tree
	app.ValUpdates = append(app.ValUpdates, v)

	return types.ResultDeliverTx{Code: code.CodeTypeOK}
}
