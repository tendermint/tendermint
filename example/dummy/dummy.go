package dummy

import (
	"encoding/hex"
	"strings"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-merkle"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/abci/types"
)

type DummyApplication struct {
	state merkle.Tree
}

func NewDummyApplication() *DummyApplication {
	state := merkle.NewIAVLTree(0, nil)
	return &DummyApplication{state: state}
}

func (app *DummyApplication) Info() (resInfo types.ResponseInfo) {
	return types.ResponseInfo{Data: Fmt("{\"size\":%v}", app.state.Size())}
}

func (app *DummyApplication) SetOption(key string, value string) (log string) {
	return ""
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
	hash := app.state.Hash()
	return types.NewResultOK(hash, "")
}

func (app *DummyApplication) Query(query []byte) types.Result {
	index, value, exists := app.state.Get(query)
	queryResult := QueryResult{index, string(value), hex.EncodeToString(value), exists}
	return types.NewResultOK(wire.JSONBytes(queryResult), "")
}

type QueryResult struct {
	Index    int    `json:"index"`
	Value    string `json:"value"`
	ValueHex string `json:"valueHex"`
	Exists   bool   `json:"exists"`
}
