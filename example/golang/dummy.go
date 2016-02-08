package example

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

func (app *DummyApplication) AppendTx(tx []byte) (code types.CodeType, result []byte, log string) {
	parts := strings.Split(string(tx), "=")
	if len(parts) == 2 {
		app.state.Set([]byte(parts[0]), []byte(parts[1]))
	} else {
		app.state.Set(tx, tx)
	}
	return types.CodeType_OK, nil, ""
}

func (app *DummyApplication) CheckTx(tx []byte) (code types.CodeType, result []byte, log string) {
	return types.CodeType_OK, nil, ""
}

func (app *DummyApplication) GetHash() (hash []byte, log string) {
	hash = app.state.Hash()
	return hash, ""
}

func (app *DummyApplication) Query(query []byte) (code types.CodeType, result []byte, log string) {
	index, value, exists := app.state.Get(query)
	resStr := Fmt("Index=%v value=%v exists=%v", index, string(value), exists)
	return types.CodeType_OK, []byte(resStr), ""
}
