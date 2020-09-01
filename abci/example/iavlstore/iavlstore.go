package iavlstore

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cosmos/iavl"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/version"
)

var _ types.Application = (*Application)(nil)

type Application struct {
	types.BaseApplication
	store *iavl.MutableTree
}

func NewApplication(dataDir string) *Application {
	db, err := dbm.NewGoLevelDB("iavlstore", dataDir)
	if err != nil {
		panic(err)
	}
	store, err := iavl.NewMutableTree(db, 0)
	if err != nil {
		panic(err)
	}
	_, err = store.Load()
	if err != nil {
		panic(err)
	}
	return &Application{
		store: store,
	}
}

func (app *Application) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
	return types.ResponseInfo{
		Data:             fmt.Sprintf(`{"size":%v}`, app.store.Size()),
		Version:          version.ABCIVersion,
		AppVersion:       1,
		LastBlockHeight:  app.store.Version(),
		LastBlockAppHash: app.store.Hash(),
	}
}

func (app *Application) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx {
	_, _, err := parseTx(req.Tx)
	if err != nil {
		return types.ResponseCheckTx{
			Code: code.CodeTypeEncodingError,
			Log:  err.Error(),
		}
	}
	return types.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}
}

func (app *Application) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {
	key, value, err := parseTx(req.Tx)
	if err != nil {
		panic(err)
	}
	app.store.Set(key, value)
	return types.ResponseDeliverTx{Code: code.CodeTypeOK}
}

func (app *Application) Commit() types.ResponseCommit {
	hash, _, err := app.store.SaveVersion()
	if err != nil {
		panic(err)
	}
	return types.ResponseCommit{Data: hash}
}

func (app *Application) Query(req types.RequestQuery) types.ResponseQuery {
	_, value := app.store.Get(req.Data)
	return types.ResponseQuery{
		Height: app.store.Version(),
		Key:    req.Data,
		Value:  value,
	}
}

// parseTx parses a tx in 'key=value' format into a key and value.
func parseTx(tx []byte) ([]byte, []byte, error) {
	parts := bytes.Split(tx, []byte("="))
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("invalid tx format: %q", string(tx))
	}
	if len(parts[0]) == 0 {
		return nil, nil, errors.New("key cannot be empty")
	}
	return parts[0], parts[1], nil
}
