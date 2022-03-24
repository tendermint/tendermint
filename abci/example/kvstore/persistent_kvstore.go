package kvstore

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/gogo/protobuf/proto"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/libs/log"
	cryptoproto "github.com/tendermint/tendermint/proto/tendermint/crypto"
	tdtypes "github.com/tendermint/tendermint/types"
)

const (
	ValidatorSetChangePrefix          string = "val:"
	ValidatorThresholdPublicKeyPrefix string = "tpk"
	ValidatorSetQuorumHashPrefix      string = "vqh"
	ValidatorSetUpdatePrefix          string = "vsu:"
)

//-----------------------------------------

var _ types.Application = (*PersistentKVStoreApplication)(nil)

var decodeBase64 = base64.StdEncoding.DecodeString

type PersistentKVStoreApplication struct {
	mtx sync.Mutex

	app *Application

	ValidatorSetUpdates types.ValidatorSetUpdate

	valProTxHashToPubKeyMap map[string]*cryptoproto.PublicKey

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
		valProTxHashToPubKeyMap: make(map[string]*cryptoproto.PublicKey),
		logger:                  log.NewNopLogger(),
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
		err := app.execValidatorSet(req.Tx)
		if err != nil {
			return types.ResponseDeliverTx{
				Code: code.CodeTypeEncodingError,
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
		key := []byte(valSetTxKey(string(reqQuery.Data)))
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

// InitChain saves the validators in the merkle tree
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

func (app *PersistentKVStoreApplication) ValidatorSet() (validatorSet types.ValidatorSetUpdate) {
	itr, err := app.app.state.db.Iterator(nil, nil)
	if err != nil {
		panic(err)
	}
	for ; itr.Valid(); itr.Next() {
		key := itr.Key()
		switch {
		case isValidatorTx(key):
			validator := new(types.ValidatorUpdate)
			err := types.ReadMessage(bytes.NewBuffer(itr.Value()), validator)
			if err != nil {
				panic(err)
			}
			validatorSet.ValidatorUpdates = append(validatorSet.ValidatorUpdates, *validator)
		case isThresholdPublicKeyTx(key):
			thresholdPublicKeyMessage := new(types.ThresholdPublicKeyUpdate)
			err := types.ReadMessage(bytes.NewBuffer(itr.Value()), thresholdPublicKeyMessage)
			if err != nil {
				panic(err)
			}
			validatorSet.ThresholdPublicKey = thresholdPublicKeyMessage.GetThresholdPublicKey()
		case isQuorumHashTx(key):
			quorumHashMessage := new(types.QuorumHashUpdate)
			err := types.ReadMessage(bytes.NewBuffer(itr.Value()), quorumHashMessage)
			if err != nil {
				panic(err)
			}
			validatorSet.QuorumHash = quorumHashMessage.GetQuorumHash()
		}
	}
	if err = itr.Error(); err != nil {
		panic(err)
	}
	return validatorSet
}

// valSetTxKey generates a key for validator set change transaction, like:
//   `val:BASE64ENCODINGOFPROTXHASH`
func valSetTxKey(proTxHashBase64 string) string {
	return ValidatorSetChangePrefix + proTxHashBase64
}

// MakeValidatorSetUpdateTx returns a transaction for updating validator-set in abci application
func MakeValidatorSetUpdateTx(
	proTxHashes []crypto.ProTxHash,
	pubKeys []crypto.PubKey,
	thresholdPubKey crypto.PubKey,
	quorumHash crypto.QuorumHash,
) ([]byte, error) {
	buf := bytes.NewBufferString(ValidatorSetUpdatePrefix)
	_, err := fmt.Fprintf(
		buf,
		"%s|%s",
		base64.StdEncoding.EncodeToString(thresholdPubKey.Bytes()),
		base64.StdEncoding.EncodeToString(quorumHash),
	)
	if err != nil {
		return nil, err
	}
	for i, proTxHash := range proTxHashes {
		var (
			pubKey []byte
			power  int64 = 0
		)
		if i < len(pubKeys) {
			pubKey = pubKeys[i].Bytes()
			power = tdtypes.DefaultDashVotingPower
		}
		_, err = fmt.Fprintf(
			buf,
			"|%s!%s!%d",
			base64.StdEncoding.EncodeToString(proTxHash.Bytes()),
			base64.StdEncoding.EncodeToString(pubKey),
			power,
		)
		if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func isValidatorSetUpdateTx(tx []byte) bool {
	return strings.HasPrefix(string(tx), ValidatorSetUpdatePrefix)
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

func (app *PersistentKVStoreApplication) execValidatorSet(tx []byte) error {
	tx = tx[len(ValidatorSetUpdatePrefix):]
	values := bytes.Split(tx, []byte{'|'})
	thresholdPubKey, err := base64.StdEncoding.DecodeString(string(values[0]))
	if err != nil {
		return fmt.Errorf("threshold Pubkey (%s) is invalid base64", string(values[0]))
	}
	app.updateThresholdPublicKey(types.UpdateThresholdPublicKey(thresholdPubKey))
	quorumHash, err := base64.StdEncoding.DecodeString(string(values[1]))
	if err != nil {
		return fmt.Errorf("quorum Hash (%s) is invalid base64", string(values[1]))
	}
	app.updateQuorumHash(types.UpdateQuorumHash(quorumHash))
	for _, val := range values[2:] {
		vals := bytes.Split(val, []byte{'!'})
		if len(vals) != 3 {
			return fmt.Errorf("expected 'proTxHash!pubkey!power'. got %v", string(val))
		}
		proTxHash, err := decodeBase64(string(vals[0]))
		if err != nil {
			return fmt.Errorf("proTxHash is invalid: %w", err)
		}
		var pubkey []byte
		if len(vals[1]) > 0 {
			pubkey, err = decodeBase64(string(vals[1]))
			if err != nil {
				return fmt.Errorf("pubkey is invalid: %w", err)
			}
		}
		// decode the power
		power, err := strconv.ParseInt(string(vals[2]), 10, 64)
		if err != nil {
			return fmt.Errorf("power (%s) is not an int", vals[2])
		}
		app.updateValidatorSet(types.UpdateValidator(proTxHash, pubkey, power, ""))
	}
	return nil
}

// add, update, or remove a validator
func (app *PersistentKVStoreApplication) updateValidatorSet(v types.ValidatorUpdate) types.ResponseDeliverTx {
	if v.PubKey != nil {
		_, err := cryptoenc.PubKeyFromProto(*v.PubKey)
		if err != nil {
			panic(fmt.Errorf("can't decode public key: %w", err))
		}
	}

	if v.ProTxHash == nil {
		panic(fmt.Errorf("proTxHash can not be nil"))
	}
	key := []byte(valSetTxKey(string(v.ProTxHash)))

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
		if err := app.app.state.db.Set(key, value.Bytes()); err != nil {
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
