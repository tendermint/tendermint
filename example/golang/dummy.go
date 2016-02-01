package example

import (
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

func (app *DummyApplication) AppendTx(tx []byte) (code types.RetCode, result []byte, log string) {
	app.state.Set(tx, tx)
	return types.RetCodeOK, nil, ""
}

func (app *DummyApplication) CheckTx(tx []byte) (code types.RetCode, result []byte, log string) {
	return types.RetCodeOK, nil, ""
}

func (app *DummyApplication) GetHash() (hash []byte, log string) {
	hash = app.state.Hash()
	return hash, ""
}

func (app *DummyApplication) Query(query []byte) (code types.RetCode, result []byte, log string) {
	return types.RetCodeOK, nil, "Query not supported"
}
