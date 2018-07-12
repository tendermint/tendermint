package kvstore

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/tendermint/iavl"
	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
)

var (
	stateKey = []byte("stateKey")
)

type State struct {
	db   dbm.DB
	tree *iavl.VersionedTree

	Size    int64  `json:"size"`
	Height  int64  `json:"height"`
	AppHash []byte `json:"app_hash"`
}

func loadState(db dbm.DB) *State {
	stateBytes := db.Get(stateKey)
	var state = new(State)
	if len(stateBytes) != 0 {
		err := json.Unmarshal(stateBytes, &state)
		if err != nil {
			panic(err)
		}
	}
	state.db = db
	state.tree = iavl.NewVersionedTree(db, 0)
	state.tree.LoadVersion(state.Height)
	return state
}

func saveState(state *State) {
	hash, version, err := state.tree.SaveVersion()
	if err != nil {
		panic(err)
	}
	state.AppHash = hash
	if state.Height+1 != version {
		panic("should not happen, expected version to be 1+state.Height.")
	}
	state.Height = version

	stateBytes, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	state.db.Set(stateKey, stateBytes)
}

//---------------------------------------------------

var _ types.Application = (*KVStoreApplication)(nil)

type KVStoreApplication struct {
	types.BaseApplication

	state *State
}

func NewKVStoreApplication() *KVStoreApplication {
	state := loadState(dbm.NewMemDB())
	return &KVStoreApplication{state: state}
}

func (app *KVStoreApplication) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
	return types.ResponseInfo{Data: fmt.Sprintf("{\"size\":%v}", app.state.Size)}
}

// tx is either "key=value" or just arbitrary bytes
func (app *KVStoreApplication) DeliverTx(tx []byte) types.ResponseDeliverTx {
	var key, value []byte
	parts := bytes.Split(tx, []byte("="))
	if len(parts) == 2 {
		key, value = parts[0], parts[1]
	} else {
		key, value = tx, tx
	}
	app.state.tree.Set(key, value)
	app.state.Size += 1

	tags := []cmn.KVPair{
		{[]byte("app.creator"), []byte("Cosmoshi Netowoko")},
		{[]byte("app.key"), key},
	}
	return types.ResponseDeliverTx{Code: code.CodeTypeOK, Tags: tags}
}

func (app *KVStoreApplication) CheckTx(tx []byte) types.ResponseCheckTx {
	return types.ResponseCheckTx{Code: code.CodeTypeOK}
}

func (app *KVStoreApplication) Commit() types.ResponseCommit {
	saveState(app.state)
	return types.ResponseCommit{Data: app.state.AppHash}
}

func (app *KVStoreApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	key := reqQuery.Data
	height := app.state.Height
	if reqQuery.Height != 0 {
		height = reqQuery.Height
	}
	if reqQuery.Prove {
		value, proof, err := app.state.tree.GetVersionedWithProof(key, height)
		if err != nil {
			resQuery.Code = code.CodeTypeUnknownError
			resQuery.Log = err.Error()
			return
		}
		resQuery.Height = height
		resQuery.Index = proof.LeftIndex() // TODO make Proof return index
		resQuery.Key = key
		resQuery.Value = value
		if value != nil {
			resQuery.Proof = &merkle.Proof{
				// XXX key encoding
				Ops: []merkle.ProofOp{iavl.NewIAVLValueOp(string(key), proof).ProofOp()},
			}
			resQuery.Log = "exists"
		} else {
			resQuery.Proof = &merkle.Proof{
				// XXX key encoding
				Ops: []merkle.ProofOp{iavl.NewIAVLAbsenceOp(string(key), proof).ProofOp()},
			}
			resQuery.Log = "does not exist"
		}
		return
	} else {
		index, value := app.state.tree.GetVersioned(key, height)
		resQuery.Height = height
		resQuery.Index = int64(index) // TODO GetVersioned64?
		resQuery.Key = key
		resQuery.Value = value
		resQuery.Proof = nil
		if value != nil {
			resQuery.Log = "exists"
		} else {
			resQuery.Log = "does not exist"
		}
		return
	}
}
