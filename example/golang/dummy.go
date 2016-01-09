package example

import (
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-merkle"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tmsp/types"
)

type DummyApplication struct {
	state merkle.Tree
}

func NewDummyApplication() *DummyApplication {
	state := merkle.NewIAVLTree(
		wire.BasicCodec,
		wire.BasicCodec,
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
	return 0
}

func (app *DummyApplication) AppendTx(tx []byte) ([]types.Event, types.RetCode) {
	app.state.Set(tx, tx)
	return nil, 0
}

func (app *DummyApplication) CheckTx(tx []byte) types.RetCode {
	return 0 // all txs are valid
}

func (app *DummyApplication) GetHash() ([]byte, types.RetCode) {
	hash := app.state.Hash()
	return hash, 0
}

func (app *DummyApplication) AddListener(key string) types.RetCode {
	return 0
}

func (app *DummyApplication) RemListener(key string) types.RetCode {
	return 0
}
