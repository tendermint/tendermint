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

func (app *DummyApplication) Echo(message string) string {
	return message
}

func (app *DummyApplication) Info() []string {
	return []string{Fmt("size:%v", app.state.Size())}
}

func (app *DummyApplication) SetOption(key string, value string) types.RetCode {
	return types.RetCodeOK
}

func (app *DummyApplication) AppendTx(tx []byte) ([]types.Event, types.RetCode) {
	app.state.Set(tx, tx)
	return nil, types.RetCodeOK
}

func (app *DummyApplication) CheckTx(tx []byte) types.RetCode {
	return types.RetCodeOK // all txs are valid
}

func (app *DummyApplication) GetHash() ([]byte, types.RetCode) {
	hash := app.state.Hash()
	return hash, types.RetCodeOK
}

func (app *DummyApplication) AddListener(key string) types.RetCode {
	return types.RetCodeOK
}

func (app *DummyApplication) RemListener(key string) types.RetCode {
	return types.RetCodeOK
}

func (app *DummyApplication) Query(query []byte) (types.RetCode, []byte) {
	return types.RetCodeOK, nil
}
