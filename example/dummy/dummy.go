package dummy

import (
	"strings"

	"github.com/tendermint/abci/types"
	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/iavl"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
)

type DummyApplication struct {
	types.BaseApplication

	state *iavl.VersionedTree
}

func NewDummyApplication() *DummyApplication {
	state := iavl.NewVersionedTree(0, dbm.NewMemDB())
	return &DummyApplication{state: state}
}

func (app *DummyApplication) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
	return types.ResponseInfo{Data: cmn.Fmt("{\"size\":%v}", app.state.Size())}
}

// tx is either "key=value" or just arbitrary bytes
func (app *DummyApplication) DeliverTx(tx []byte) types.Result {
	parts := strings.Split(string(tx), "=")
	if len(parts) == 2 {
		app.state.Set([]byte(parts[0]), []byte(parts[1]))
	} else {
		app.state.Set(tx, tx)
	}
	return types.OK
}

func (app *DummyApplication) CheckTx(tx []byte) types.Result {
	return types.OK
}

func (app *DummyApplication) Commit() types.Result {
	// Save a new version
	var hash []byte
	var err error

	if app.state.Size() > 0 {
		// just add one more to height (kind of arbitrarily stupid)
		height := app.state.LatestVersion() + 1
		hash, err = app.state.SaveVersion(height)
		if err != nil {
			// if this wasn't a dummy app, we'd do something smarter
			panic(err)
		}
	}

	return types.NewResultOK(hash, "")
}

func (app *DummyApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	if reqQuery.Prove {
		value, proof, err := app.state.GetWithProof(reqQuery.Data)
		// if this wasn't a dummy app, we'd do something smarter
		if err != nil {
			panic(err)
		}
		resQuery.Index = -1 // TODO make Proof return index
		resQuery.Key = reqQuery.Data
		resQuery.Value = value
		resQuery.Proof = wire.BinaryBytes(proof)
		if value != nil {
			resQuery.Log = "exists"
		} else {
			resQuery.Log = "does not exist"
		}
		return
	} else {
		index, value := app.state.Get(reqQuery.Data)
		resQuery.Index = int64(index)
		resQuery.Value = value
		if value != nil {
			resQuery.Log = "exists"
		} else {
			resQuery.Log = "does not exist"
		}
		return
	}
}
