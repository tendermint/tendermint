package kvstore

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/tendermint/tendermint/crypto"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/types"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/libs/log"
	pc "github.com/tendermint/tendermint/proto/tendermint/crypto"
)

const (
	ValidatorSetChangePrefix                string = "val:"
	ValidatorThresholdPublicKeyChangePrefix string = "tpk:"
	ValidatorThresholdPublicKeyPrefix       string = "tpk"
	ValidatorSetQuorumHashPrefix            string = "vqh"
	ValidatorSetQuorumHashChangePrefix      string = "vqh:"
)

//-----------------------------------------

var _ types.Application = (*PersistentKVStoreApplication)(nil)

type PersistentKVStoreApplication struct {
	app *Application

	ValidatorSetUpdates types.ValidatorSetUpdate

	valProTxHashToPubKeyMap map[string]pc.PublicKey

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
		app:                     &Application{state: state},
		valProTxHashToPubKeyMap: make(map[string]pc.PublicKey),
		logger:                  log.NewNopLogger(),
	}
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

func (app *PersistentKVStoreApplication) SetOption(req types.RequestSetOption) types.ResponseSetOption {
	return app.app.SetOption(req)
}

// tx is either "val:proTxHash!pubkey!power" or "key=value" or just arbitrary bytes
func (app *PersistentKVStoreApplication) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {
	// if it starts with "vals:", update the validator set
	// format is "val:proTxHash!pubkey!power"
	if isValidatorTx(req.Tx) {
		// update validators in the merkle tree
		// and in app.ValUpdates
		return app.execValidatorTx(req.Tx)
	} else if isThresholdPublicKeyTx(req.Tx) {
		return app.execThresholdPublicKeyTx(req.Tx)
	} else if isQuorumHashTx(req.Tx) {
		return app.execQuorumHashTx(req.Tx)
	}

	// otherwise, update the key-value store
	return app.app.DeliverTx(req)
}

func (app *PersistentKVStoreApplication) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx {
	return app.app.CheckTx(req)
}

// Commit will panic if InitChain was not called
func (app *PersistentKVStoreApplication) Commit() types.ResponseCommit {
	return app.app.Commit()
}

// When path=/val and data={validator address}, returns the validator update (types.ValidatorUpdate) varint encoded.
// For any other path, returns an associated value or nil if missing.
func (app *PersistentKVStoreApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	switch reqQuery.Path {
	case "/verify-chainlock":
		resQuery.Code = 0

		return resQuery
	case "/val":
		key := []byte("val:" + string(reqQuery.Data))
		value, err := app.app.state.db.Get(key)
		if err != nil {
			panic(err)
		}

		resQuery.Key = reqQuery.Data
		resQuery.Value = value
		return
	case "/tpk":
		key := []byte("tpk")
		value, err := app.app.state.db.Get(key)
		if err != nil {
			panic(err)
		}

		resQuery.Key = reqQuery.Data
		resQuery.Value = value
		return
	case "/vqh":
		key := []byte("vqh")
		value, err := app.app.state.db.Get(key)
		if err != nil {
			panic(err)
		}

		resQuery.Key = reqQuery.Data
		resQuery.Value = value
		return
	default:
		return app.app.Query(reqQuery)
	}
}

// Save the validators in the merkle tree
func (app *PersistentKVStoreApplication) InitChain(req types.RequestInitChain) types.ResponseInitChain {
	for _, v := range req.ValidatorSet.ValidatorUpdates {
		r := app.updateValidatorSet(v)
		if r.IsErr() {
			app.logger.Error("Error updating validators", "r", r)
		}
	}
	app.updateThresholdPublicKey(types.ThresholdPublicKeyUpdate{
		ThresholdPublicKey: req.ValidatorSet.ThresholdPublicKey,
	})
	app.updateQuorumHash(types.QuorumHashUpdate{
		QuorumHash: req.ValidatorSet.QuorumHash,
	})
	return types.ResponseInitChain{}
}

// Track the block hash and header information
func (app *PersistentKVStoreApplication) BeginBlock(req types.RequestBeginBlock) types.ResponseBeginBlock {
	// reset valset changes
	app.ValidatorSetUpdates.ValidatorUpdates = make([]types.ValidatorUpdate, 0)

	return types.ResponseBeginBlock{}
}

// Update the validator set
func (app *PersistentKVStoreApplication) EndBlock(req types.RequestEndBlock) types.ResponseEndBlock {
	return types.ResponseEndBlock{ValidatorSetUpdate: &app.ValidatorSetUpdates}
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

func (app *PersistentKVStoreApplication) ValidatorSet() (validatorSet types.ValidatorSetUpdate) {
	itr, err := app.app.state.db.Iterator(nil, nil)
	if err != nil {
		panic(err)
	}
	for ; itr.Valid(); itr.Next() {
		key := itr.Key()
		if isValidatorTx(key) {
			validator := new(types.ValidatorUpdate)
			err := types.ReadMessage(bytes.NewBuffer(itr.Value()), validator)
			if err != nil {
				panic(err)
			}
			validatorSet.ValidatorUpdates = append(validatorSet.ValidatorUpdates, *validator)
		} else if isThresholdPublicKeyTx(key) {
			thresholdPublicKeyMessage := new(types.ThresholdPublicKeyUpdate)
			err := types.ReadMessage(bytes.NewBuffer(itr.Value()), thresholdPublicKeyMessage)
			if err != nil {
				panic(err)
			}
			validatorSet.ThresholdPublicKey = thresholdPublicKeyMessage.GetThresholdPublicKey()
		}
	}
	if err = itr.Error(); err != nil {
		panic(err)
	}
	return
}

func MakeValSetChangeTx(proTxHash []byte, pubkey pc.PublicKey, power int64) []byte {
	pk, err := cryptoenc.PubKeyFromProto(pubkey)
	if err != nil {
		panic(err)
	}
	pubStr := base64.StdEncoding.EncodeToString(pk.Bytes())
	proTxHashStr := base64.StdEncoding.EncodeToString(proTxHash)
	return []byte(fmt.Sprintf("val:%s!%s!%d", proTxHashStr, pubStr, power))
}

func MakeThresholdPublicKeyChangeTx(thresholdPublicKey pc.PublicKey) []byte {
	pk, err := cryptoenc.PubKeyFromProto(thresholdPublicKey)
	if err != nil {
		panic(err)
	}
	pubStr := base64.StdEncoding.EncodeToString(pk.Bytes())
	return []byte(fmt.Sprintf("tpk:%s", pubStr))
}

func MakeQuorumHashTx(quorumHash crypto.QuorumHash) []byte {
	pubStr := base64.StdEncoding.EncodeToString(quorumHash)
	return []byte(fmt.Sprintf("vqh:%s", pubStr))
}

func isValidatorTx(tx []byte) bool {
	return strings.HasPrefix(string(tx), ValidatorSetChangePrefix)
}

func isThresholdPublicKeyTx(tx []byte) bool {
	return strings.HasPrefix(string(tx), ValidatorThresholdPublicKeyPrefix)
}

func isQuorumHashTx(tx []byte) bool {
	return strings.HasPrefix(string(tx), ValidatorSetQuorumHashPrefix)
}

// format is "val:proTxHash!pubkey!power"
// pubkey is a base64-encoded 48-byte bls12381 key
func (app *PersistentKVStoreApplication) execValidatorTx(tx []byte) types.ResponseDeliverTx {
	tx = tx[len(ValidatorSetChangePrefix):]

	//  get the pubkey and power
	values := strings.Split(string(tx), "!")
	if len(values) != 3 {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Expected 'proTxHash!pubkey!power'. Got %v", values)}
	}
	proTxHashS, pubkeyS, powerS := values[0], values[1], values[2]

	// decode the proTxHash
	proTxHash, err := base64.StdEncoding.DecodeString(proTxHashS)
	if err != nil {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("ProTxHash (%s) is invalid base64", proTxHash)}
	}

	// decode the pubkey
	pubkey, err := base64.StdEncoding.DecodeString(pubkeyS)
	if err != nil {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Pubkey (%s) is invalid base64", pubkeyS)}
	}

	// decode the power
	power, err := strconv.ParseInt(powerS, 10, 64)
	if err != nil {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Power (%s) is not an int", powerS)}
	}

	return app.updateValidatorSet(types.UpdateValidator(proTxHash, pubkey, power))
}

// format is "tpk:pubkey"
// pubkey is a base64-encoded 48-byte bls12381 key
func (app *PersistentKVStoreApplication) execThresholdPublicKeyTx(tx []byte) types.ResponseDeliverTx {
	tx = tx[len(ValidatorThresholdPublicKeyChangePrefix):]

	// decode the pubkey
	pubkey, err := base64.StdEncoding.DecodeString(string(tx))
	if err != nil {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Threshold Pubkey (%s) is invalid base64", string(tx))}
	}

	return app.updateThresholdPublicKey(types.UpdateThresholdPublicKey(pubkey))
}

// format is "vqh:hash"
// hash is a base64-encoded 32-byte hash
func (app *PersistentKVStoreApplication) execQuorumHashTx(tx []byte) types.ResponseDeliverTx {
	tx = tx[len(ValidatorSetQuorumHashChangePrefix):]

	// decode the pubkey
	quorumHash, err := base64.StdEncoding.DecodeString(string(tx))
	if err != nil {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Quorum Hash (%s) is invalid base64", string(tx))}
	}

	return app.updateQuorumHash(types.UpdateQuorumHash(quorumHash))
}

// add, update, or remove a validator
func (app *PersistentKVStoreApplication) updateValidatorSet(v types.ValidatorUpdate) types.ResponseDeliverTx {
	_, err := cryptoenc.PubKeyFromProto(v.PubKey)
	if v.ProTxHash == nil {
		panic(fmt.Errorf("proTxHash can not be nil: %w", err))
	}
	if err != nil {
		panic(fmt.Errorf("can't decode public key: %w", err))
	}
	key := []byte("val:" + string(v.ProTxHash))

	if v.Power == 0 {
		// remove validator
		hasKey, err := app.app.state.db.Has(key)
		if err != nil {
			panic(err)
		}
		if !hasKey {
			proTxHashString := base64.StdEncoding.EncodeToString(v.ProTxHash)
			return types.ResponseDeliverTx{
				Code: code.CodeTypeUnauthorized,
				Log:  fmt.Sprintf("Cannot remove non-existent validator %s", proTxHashString)}
		}
		if err = app.app.state.db.Delete(key); err != nil {
			panic(err)
		}
		delete(app.valProTxHashToPubKeyMap, string(v.ProTxHash))
	} else {
		// add or update validator
		value := bytes.NewBuffer(make([]byte, 0))
		if err := types.WriteMessage(&v, value); err != nil {
			return types.ResponseDeliverTx{
				Code: code.CodeTypeEncodingError,
				Log:  fmt.Sprintf("Error encoding validator: %v", err)}
		}
		if err = app.app.state.db.Set(key, value.Bytes()); err != nil {
			panic(err)
		}
		app.valProTxHashToPubKeyMap[string(v.ProTxHash)] = v.PubKey
	}

	// we only update the changes array if we successfully updated the tree
	app.ValidatorSetUpdates.ValidatorUpdates = append(app.ValidatorSetUpdates.ValidatorUpdates, v)

	return types.ResponseDeliverTx{Code: code.CodeTypeOK}
}

func (app *PersistentKVStoreApplication) updateThresholdPublicKey(
	thresholdPublicKeyUpdate types.ThresholdPublicKeyUpdate) types.ResponseDeliverTx {
	_, err := cryptoenc.PubKeyFromProto(thresholdPublicKeyUpdate.ThresholdPublicKey)
	if err != nil {
		panic(fmt.Errorf("can't decode threshold public key: %w", err))
	}
	key := []byte("tpk")

	// add or update thresholdPublicKey
	value := bytes.NewBuffer(make([]byte, 0))
	if err := types.WriteMessage(&thresholdPublicKeyUpdate, value); err != nil {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Error encoding threshold public key: %v", err)}
	}
	if err = app.app.state.db.Set(key, value.Bytes()); err != nil {
		panic(err)
	}

	// we only update the changes array if we successfully updated the tree
	app.ValidatorSetUpdates.ThresholdPublicKey = thresholdPublicKeyUpdate.GetThresholdPublicKey()

	return types.ResponseDeliverTx{Code: code.CodeTypeOK}
}

func (app *PersistentKVStoreApplication) updateQuorumHash(
	quorumHashUpdate types.QuorumHashUpdate) types.ResponseDeliverTx {
	key := []byte("vqh")

	// add or update thresholdPublicKey
	value := bytes.NewBuffer(make([]byte, 0))
	if err := types.WriteMessage(&quorumHashUpdate, value); err != nil {
		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("Error encoding threshold public key: %v", err)}
	}
	if err := app.app.state.db.Set(key, value.Bytes()); err != nil {
		panic(err)
	}

	// we only update the changes array if we successfully updated the tree
	app.ValidatorSetUpdates.QuorumHash = quorumHashUpdate.GetQuorumHash()

	return types.ResponseDeliverTx{Code: code.CodeTypeOK}
}
