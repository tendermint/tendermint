package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosmos/iavl"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/code"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/version"
)

// Transaction type bytes
const (
	TxTypeSet           byte = 0x01
	TxTypeRm            byte = 0x02
	TxTypeGet           byte = 0x03
	TxTypeCompareAndSet byte = 0x04
	TxTypeValSetChange  byte = 0x05
	TxTypeValSetRead    byte = 0x06
	TxTypeValSetCAS     byte = 0x07

	NonceLength = 12

	// Additional codes.
	CodeTypeUnknownRequest        = 2
	CodeTypeEncodingError         = 3
	CodeTypeBadNonce              = 4
	CodeTypeErrUnknownRequest     = 5
	CodeTypeInternalError         = 6
	CodeTypeErrBaseUnknownAddress = 7
	CodeTypeErrUnauthorized       = 8
)

// Database key for merkle tree save value db values
var eyesStateKey = []byte("merkleeyes:state")

// MerkleEyesApp is a Merkle KV-store served as an ABCI app.
type MerkleEyesApp struct {
	abci.BaseApplication

	state State
	db    dbm.DB

	height     int64
	validators *ValidatorSetState

	changes []abci.ValidatorUpdate
}

var _ abci.Application = (*MerkleEyesApp)(nil)

// MerkleEyesState contains the latest Merkle root hash.
type MerkleEyesState struct {
	Hash       []byte             `json:"hash"`
	Height     int64              `json:"height"`
	Validators *ValidatorSetState `json:"validators"`
}

type ValidatorSetState struct {
	Version    uint64       `json:"version"`
	Validators []*Validator `json:"validators"`
}

// Validator represents a single validator.
type Validator struct {
	PubKey crypto.PubKey `json:"pub_key"`
	Power  int64         `json:"power"`
}

// Has returns true if v is present in the validator set.
func (vss *ValidatorSetState) Has(v *Validator) bool {
	for _, v1 := range vss.Validators {
		if v1.PubKey.Equals(v.PubKey) {
			return true
		}
	}
	return false
}

// Remove removes v from the validator set.
func (vss *ValidatorSetState) Remove(v *Validator) {
	vals := make([]*Validator, 0, len(vss.Validators)-1)
	for _, v1 := range vss.Validators {
		if v1.PubKey.Equals(v.PubKey) {
			vals = append(vals, v1)
		}
	}
	vss.Validators = vals
}

// Set updates v.
func (vss *ValidatorSetState) Set(v *Validator) {
	for i, v1 := range vss.Validators {
		if v1.PubKey.Equals(v.PubKey) {
			vss.Validators[i] = v
			return
		}
	}
	vss.Validators = append(vss.Validators, v)
}

// NewMerkleEyesApp initializes the database, loads any existing state, and
// returns a new MerkleEyesApp.
func NewMerkleEyesApp(dbName string, cacheSize int) *MerkleEyesApp {
	// start at 1 so the height returned by query is for the next block, ie. the
	// one that includes the AppHash for our current state
	initialHeight := int64(1)

	// Non-persistent case
	if dbName == "" {
		tree, err := iavl.NewMutableTree(dbm.NewMemDB(), cacheSize)
		if err != nil {
			panic(err)
		}

		return &MerkleEyesApp{
			state:  NewState(tree, 0, false),
			db:     nil,
			height: initialHeight,
		}
	}

	// Setup the persistent merkle tree
	var empty bool
	_, err := os.Stat(filepath.Join(dbName, dbName+".db"))
	if err != nil && os.IsNotExist(err) {
		empty = true
	}

	// Open the db, if the db doesn't exist it will be created
	db, err := dbm.NewGoLevelDB(dbName, dbName)
	if err != nil {
		panic(err)
	}

	// Load Tree
	tree, err := iavl.NewMutableTree(db, cacheSize)
	if err != nil {
		panic(err)
	}

	if empty {
		fmt.Println("no existing db, creating new db")
		bz, err := json.Marshal(MerkleEyesState{
			Hash:   tree.Hash(),
			Height: initialHeight,
		})
		if err != nil {
			panic(err)
		}
		db.Set(eyesStateKey, bz)
	} else {
		fmt.Println("loading existing db")
	}

	// Load merkle state
	eyesStateBytes, err := db.Get(eyesStateKey)
	if err != nil {
		panic(err)
	}
	var eyesState MerkleEyesState
	err = json.Unmarshal(eyesStateBytes, &eyesState)
	if err != nil {
		fmt.Println("error reading MerkleEyesState")
		panic(err)
	}

	version, err := tree.Load()
	if err != nil {
		panic(err)
	}

	return &MerkleEyesApp{
		state:      NewState(tree, version, true),
		db:         db,
		height:     eyesState.Height,
		validators: new(ValidatorSetState),
	}
}

// CloseDB closes the database
func (app *MerkleEyesApp) CloseDB() {
	if app.db != nil {
		app.db.Close()
	}
}

// Info implements ABCI.
func (app *MerkleEyesApp) Info(req abci.RequestInfo) abci.ResponseInfo {
	return abci.ResponseInfo{
		Version:          version.ABCIVersion,
		AppVersion:       1,
		LastBlockHeight:  app.height,
		LastBlockAppHash: app.state.Deliver().Hash(),
	}
}

// InitChain implements ABCI.
func (app *MerkleEyesApp) InitChain(req abci.RequestInitChain) abci.ResponseInitChain {
	for _, v := range req.Validators {
		app.validators.Set(&Validator{PubKey: ed25519.PubKey(v.PubKey.GetEd25519()), Power: v.Power})
	}

	return abci.ResponseInitChain{
		AppHash: app.state.Deliver().Hash(),
	}
}

// CheckTx implements ABCI.
func (app *MerkleEyesApp) CheckTx(_ abci.RequestCheckTx) abci.ResponseCheckTx {
	return abci.ResponseCheckTx{Code: code.CodeTypeOK}
}

// DeliverTx implements ABCI.
func (app *MerkleEyesApp) DeliverTx(req abci.RequestDeliverTx) abci.ResponseDeliverTx {
	tree := app.state.Deliver()
	r := app.doTx(tree, req.Tx)
	if r.IsErr() {
		fmt.Println("DeliverTx Err", r)
	}
	return r
}

// BeginBlock implements ABCI.
func (app *MerkleEyesApp) BeginBlock(req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	// reset valset changes
	app.changes = make([]abci.ValidatorUpdate, 0)
	return abci.ResponseBeginBlock{}
}

// EndBlock implements ABCI.
func (app *MerkleEyesApp) EndBlock(req abci.RequestEndBlock) abci.ResponseEndBlock {
	if len(app.changes) > 0 {
		app.validators.Version++
	}
	return abci.ResponseEndBlock{ValidatorUpdates: app.changes}
}

// Commit implements abci.Application
func (app *MerkleEyesApp) Commit() abci.ResponseCommit {
	hash := app.state.Commit()

	app.height++

	if app.db != nil {
		bz, err := json.Marshal(MerkleEyesState{
			Hash:       hash,
			Height:     app.height,
			Validators: app.validators,
		})
		if err != nil {
			panic(err)
		}
		app.db.Set(eyesStateKey, bz)
	}

	// if app.state.Committed().Size() == 0 {
	// 	return abci.ResponseCommit{Data: nil}
	// }
	return abci.ResponseCommit{Data: hash}
}

// Query implements ABCI.
func (app *MerkleEyesApp) Query(req abci.RequestQuery) (res abci.ResponseQuery) {
	if len(req.Data) == 0 {
		return
	}
	// tree := app.state.Committed()
	tree := app.state.Deliver()

	if req.Height != 0 {
		// TODO: support older commits
		res.Code = CodeTypeInternalError
		res.Log = "merkleeyes only supports queries on latest commit"
		return
	}

	// set the query response height
	res.Height = app.height

	switch req.Path {

	case "/store", "/key": // Get by key
		key := req.Data // Data holds the key bytes
		res.Key = key
		if req.Prove {
			value, _, err := tree.GetWithProof(storeKey(key))
			if err != nil {
				res.Log = "Key not found"
			}
			res.Value = value
			// FIXME: construct proof op and return it
			// res.Proof = proof
			// TODO: return index too?
		} else {
			index, value := tree.Get(storeKey(key))
			res.Value = value
			res.Index = int64(index)
		}

	case "/index": // Get by Index
		index := wire.GetInt64(req.Data)
		key, value := tree.GetByIndex(int(index))
		res.Key = key
		res.Index = int64(index)
		res.Value = value

	case "/size": // Get size
		size := tree.Size()
		sizeBytes := wire.BinaryBytes(size)
		res.Value = sizeBytes

	default:
		res.Code = CodeType_UnknownRequest
		res.Log = fmt.Sprintf("Unexpected Query path: %v", req.Path)
	}
	return
}

func nonceKey(nonce []byte) []byte {
	return append([]byte("/nonce/"), nonce...)
}

func storeKey(key []byte) []byte {
	return append([]byte("/key/"), key...)
}

func (app *MerkleEyesApp) doTx(tree *iavl.MutableTree, tx []byte) abci.ResponseDeliverTx {
	// minimum length is 12 (nonce) + 1 (type byte) = 13
	minTxLen := NonceLength + 1
	if len(tx) < minTxLen {
		return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: fmt.Sprintf("Tx length must be at least %d", minTxLen)}
	}

	nonce := tx[:12]
	tx = tx[12:]

	// check nonce
	_, n := tree.Get(nonceKey(nonce))
	if n == nil {
		return abci.ResponseDeliverTx{Code: CodeTypeBadNonce, Log: fmt.Sprintf("Nonce %X already exists", nonce)}
	}

	// set nonce
	tree.Set(nonceKey(nonce), []byte("found"))

	typeByte := tx[0]
	tx = tx[1:]
	switch typeByte {
	case TxTypeSet: // Set
		key, n, err := wire.GetByteSlice(tx)
		if err != nil {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: fmt.Sprintf("Error reading key: %v", err)}
		}
		tx = tx[n:]
		value, n, err := wire.GetByteSlice(tx)
		if err != nil {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: fmt.Sprintf("Error reading value: %v", err)}
		}
		tx = tx[n:]
		if len(tx) != 0 {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: "Got bytes left over"}
		}

		tree.Set(storeKey(key), value)

		fmt.Println("SET", fmt.Sprintf("%X", key), fmt.Sprintf("%X", value))
	case TxTypeRm: // Remove
		key, n, err := wire.GetByteSlice(tx)
		if err != nil {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: fmt.Sprintf("Error reading key: %v", err)}
		}
		tx = tx[n:]
		if len(tx) != 0 {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: fmt.Sprintf("Got bytes left over")}
		}
		tree.Remove(storeKey(key))
		fmt.Println("RM", fmt.Sprintf("%X", key))
	case TxTypeGet: // Get
		key, n, err := wire.GetByteSlice(tx)
		if err != nil {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: fmt.Sprintf("Error reading key: %v", err)}
		}
		tx = tx[n:]
		if len(tx) != 0 {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: "Got bytes left over"}
		}

		_, value := tree.Get(storeKey(key))
		if value != nil {
			fmt.Println("GET", fmt.Sprintf("%X", key), fmt.Sprintf("%X", value))
			return abci.ResponseDeliverTx{Code: abci.CodeTypeOK, Data: value}
		}

		return abci.ResponseDeliverTx{Code: CodeTypeErrBaseUnknownAddress,
			Log: fmt.Sprintf("Cannot find key: %X", key)}
	case TxTypeCompareAndSet: // Compare and Set
		key, n, err := wire.GetByteSlice(tx)
		if err != nil {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: fmt.Sprintf("Error reading key: %v", err)}
		}
		tx = tx[n:]

		compareValue, n, err := wire.GetByteSlice(tx)
		if err != nil {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: fmt.Sprintf("Error reading compare value: %v", err)}
		}
		tx = tx[n:]

		setValue, n, err := wire.GetByteSlice(tx)
		if err != nil {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: fmt.Sprintf("Error reading set value: %v", err)}
		}
		tx = tx[n:]

		if len(tx) != 0 {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: "Got bytes left over"}
		}

		_, value, exists := tree.Get(storeKey(key))
		if !exists {
			return abci.ResponseDeliverTx{Code: CodeTypeErrBaseUnknownAddress,
				Log: fmt.Sprintf("Cannot find key: %X", key)}
		}
		if !bytes.Equal(value, compareValue) {
			return abci.ResponseDeliverTx{Code: CodeTypeErrUnauthorized,
				Log: fmt.Sprintf("Value was %X, not %X", value, compareValue)}
		}
		tree.Set(storeKey(key), setValue)

		fmt.Println("CAS-SET", fmt.Sprintf("%X", key), fmt.Sprintf("%X", compareValue), fmt.Sprintf("%X", setValue))
	case TxTypeValSetChange:
		pubKey, n, err := wire.GetByteSlice(tx)
		if err != nil {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError, Log: fmt.Sprintf("Error reading pubkey: %v", err)}
		}
		if len(pubKey) != 32 {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError,
				Log: fmt.Sprintf("Pubkey must be 32 bytes: %X is %d bytes", pubKey, len(pubKey))}
		}
		tx = tx[n:]
		if len(tx) != 8 {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError,
				Log: fmt.Sprintf("Power must be 8 bytes: %X is %d bytes", tx, len(tx))}
		}
		power := wire.GetUint64(tx)

		return app.updateValidator(pubKey, power)

	case TxTypeValSetRead:
		b, err := json.Marshal(app.validators)
		if err != nil {
			return abci.ResponseDeliverTx{Code: CodeTypeInternalError,
				Log: fmt.Sprintf("Error marshalling validator info: %v", err)}
		}
		return abci.ResponseDeliverTx{Code: abci.CodeTypeOK, Data: b, Log: string(b)}

	case TxTypeValSetCAS:
		if len(tx) < 8 {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError,
				Log: fmt.Sprintf("Version number must be 8 bytes: remaining tx (%X) is %d bytes", tx, len(tx))}
		}
		version := wire.GetUint64(tx)
		if app.validators.Version != version {
			return abci.ResponseDeliverTx{Code: CodeTypeErrUnauthorized,
				Log: fmt.Sprintf("Version was %d, not %d", app.validators.Version, version)}
		}
		tx = tx[8:]

		pubKey, n, err := wire.GetByteSlice(tx)
		if err != nil {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError,
				Log: fmt.Sprintf("Error reading pubkey: %v", err)}
		}
		if len(pubKey) != 32 {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError,
				Log: fmt.Sprintf("Pubkey must be 32 bytes: %X is %d bytes", pubKey, len(pubKey))}
		}
		tx = tx[n:]
		if len(tx) != 8 {
			return abci.ResponseDeliverTx{Code: CodeTypeEncodingError,
				Log: fmt.Sprintf("Power must be 8 bytes: %X is %d bytes", tx, len(tx))}
		}
		power := wire.GetUint64(tx)

		return app.updateValidator(pubKey, power)

	default:
		return abci.ResponseDeliverTx{Code: CodeTypeErrUnknownRequest,
			Log: fmt.Sprintf("Unexpected Tx type byte %X", typeByte)}
	}

	return abci.ResponseDeliverTx{Code: abci.CodeTypeOK}
}

func (app *MerkleEyesApp) updateValidator(pubKey []byte, power uint64) abci.ResponseDeliverTx {
	v := &Validator{PubKey: pubKey, Power: power}
	if v.Power == 0 {
		// remove validator
		if !app.validators.Has(v) {
			return abci.ResponseDeliverTx{Code: CodeTypeErrUnauthorized,
				Log: fmt.Sprintf("Cannot remove non-existent validator %v", v)}
		}
		app.validators.Remove(v)
	} else {
		// add or update validator
		app.validators.Set(v)
	}

	// copy to PubKeyEd25519 so we can go-wire encode properly for the changes array
	var pubKeyEd crypto.PubKeyEd25519
	copy(pubKeyEd[:], pubKey)
	app.changes = append(app.changes, &abci.Validator{pubKeyEd.Bytes(), power})

	return abci.ResponseDeliverTx{Code: abci.CodeTypeOK}
}
