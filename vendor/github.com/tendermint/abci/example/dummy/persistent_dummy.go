package dummy

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/tendermint/abci/example/code"
	"github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/iavl"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"
)

const (
	ValidatorSetChangePrefix string = "val:"
)

//-----------------------------------------

var _ types.Application = (*PersistentDummyApplication)(nil)

type PersistentDummyApplication struct {
	app *DummyApplication

	// validator set
	ValUpdates []*types.Validator

	logger log.Logger
}

func NewPersistentDummyApplication(dbDir string) *PersistentDummyApplication {
	name := "dummy"
	db, err := dbm.NewGoLevelDB(name, dbDir)
	if err != nil {
		panic(err)
	}

	stateTree := iavl.NewVersionedTree(500, db)
	stateTree.Load()

	return &PersistentDummyApplication{
		app:    &DummyApplication{state: stateTree},
		logger: log.NewNopLogger(),
	}
}

func (app *PersistentDummyApplication) SetLogger(l log.Logger) {
	app.logger = l
}

func (app *PersistentDummyApplication) Info(req types.RequestInfo) types.ResponseInfo {
	res := app.app.Info(req)
	var latestVersion uint64 = app.app.state.LatestVersion() // TODO: change to int64
	res.LastBlockHeight = int64(latestVersion)
	res.LastBlockAppHash = app.app.state.Hash()
	return res
}

func (app *PersistentDummyApplication) SetOption(req types.RequestSetOption) types.ResponseSetOption {
	return app.app.SetOption(req)
}

// tx is either "val:pubkey/power" or "key=value" or just arbitrary bytes
func (app *PersistentDummyApplication) DeliverTx(tx []byte) types.ResponseDeliverTx {
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

func (app *PersistentDummyApplication) CheckTx(tx []byte) types.ResponseCheckTx {
	return app.app.CheckTx(tx)
}

// Commit will panic if InitChain was not called
func (app *PersistentDummyApplication) Commit() types.ResponseCommit {

	// Save a new version for next height
	height := app.app.state.LatestVersion() + 1
	var appHash []byte
	var err error

	appHash, err = app.app.state.SaveVersion(height)
	if err != nil {
		// if this wasn't a dummy app, we'd do something smarter
		panic(err)
	}

	app.logger.Info("Commit block", "height", height, "root", appHash)
	return types.ResponseCommit{Code: code.CodeTypeOK, Data: appHash}
}

func (app *PersistentDummyApplication) Query(reqQuery types.RequestQuery) types.ResponseQuery {
	return app.app.Query(reqQuery)
}

// Save the validators in the merkle tree
func (app *PersistentDummyApplication) InitChain(req types.RequestInitChain) types.ResponseInitChain {
	for _, v := range req.Validators {
		r := app.updateValidator(v)
		if r.IsErr() {
			app.logger.Error("Error updating validators", "r", r)
		}
	}
	return types.ResponseInitChain{}
}

// Track the block hash and header information
func (app *PersistentDummyApplication) BeginBlock(req types.RequestBeginBlock) types.ResponseBeginBlock {
	// reset valset changes
	app.ValUpdates = make([]*types.Validator, 0)
	return types.ResponseBeginBlock{}
}

// Update the validator set
func (app *PersistentDummyApplication) EndBlock(req types.RequestEndBlock) types.ResponseEndBlock {
	return types.ResponseEndBlock{ValidatorUpdates: app.ValUpdates}
}

//---------------------------------------------
// update validators

func (app *PersistentDummyApplication) Validators() (validators []*types.Validator) {
	app.app.state.Iterate(func(key, value []byte) bool {
		if isValidatorTx(key) {
			validator := new(types.Validator)
			err := types.ReadMessage(bytes.NewBuffer(value), validator)
			if err != nil {
				panic(err)
			}
			validators = append(validators, validator)
		}
		return false
	})
	return
}

func MakeValSetChangeTx(pubkey []byte, power int64) []byte {
	return []byte(cmn.Fmt("val:%X/%d", pubkey, power))
}

func isValidatorTx(tx []byte) bool {
	return strings.HasPrefix(string(tx), ValidatorSetChangePrefix)
}

// format is "val:pubkey1/power1,addr2/power2,addr3/power3"tx
func (app *PersistentDummyApplication) execValidatorTx(tx []byte) types.ResponseDeliverTx {
	tx = tx[len(ValidatorSetChangePrefix):]

	//get the pubkey and power
	pubKeyAndPower := strings.Split(string(tx), "/")
	if len(pubKeyAndPower) != 2 {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Expected 'pubkey/power'. Got %v", pubKeyAndPower)}
	}
	pubkeyS, powerS := pubKeyAndPower[0], pubKeyAndPower[1]

	// decode the pubkey, ensuring its go-crypto encoded
	pubkey, err := hex.DecodeString(pubkeyS)
	if err != nil {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Pubkey (%s) is invalid hex", pubkeyS)}
	}
	_, err = crypto.PubKeyFromBytes(pubkey)
	if err != nil {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Pubkey (%X) is invalid go-crypto encoded", pubkey)}
	}

	// decode the power
	power, err := strconv.ParseInt(powerS, 10, 64)
	if err != nil {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Power (%s) is not an int", powerS)}
	}

	// update
	return app.updateValidator(&types.Validator{pubkey, power})
}

// add, update, or remove a validator
func (app *PersistentDummyApplication) updateValidator(v *types.Validator) types.ResponseDeliverTx {
	key := []byte("val:" + string(v.PubKey))
	if v.Power == 0 {
		// remove validator
		if !app.app.state.Has(key) {
			return types.ResponseDeliverTx{
				Code: code.CodeTypeUnauthorized,
				Log:  fmt.Sprintf("Cannot remove non-existent validator %X", key)}
		}
		app.app.state.Remove(key)
	} else {
		// add or update validator
		value := bytes.NewBuffer(make([]byte, 0))
		if err := types.WriteMessage(v, value); err != nil {
			return types.ResponseDeliverTx{
				Code: code.CodeTypeEncodingError,
				Log:  fmt.Sprintf("Error encoding validator: %v", err)}
		}
		app.app.state.Set(key, value.Bytes())
	}

	// we only update the changes array if we successfully updated the tree
	app.ValUpdates = append(app.ValUpdates, v)

	return types.ResponseDeliverTx{Code: code.CodeTypeOK}
}
