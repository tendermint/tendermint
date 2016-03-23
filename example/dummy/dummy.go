package dummy

import (
	"strings"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-merkle"
	"github.com/tendermint/tmsp/types"
)

type DummyApplication struct {
	state merkle.Tree
}

func NewDummyApplication() *DummyApplication {
	state := merkle.NewIAVLTree(
		0,
		nil,
	)
	return &DummyApplication{state: state}
}

func (app *DummyApplication) Info() string {
	return Fmt("size:%v", app.state.Size())
}

func (app *DummyApplication) SetOption(key string, value string) (log string) {
	return ""
}

func (app *DummyApplication) AppendTx(tx []byte) types.Result {
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
	resStr := Fmt("Index=%v value=%v exists=%v", index, string(value), exists)
	return types.NewResultOK([]byte(resStr), "")
}
