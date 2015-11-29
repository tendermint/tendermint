package example

import (
	"sync"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-merkle"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tmsp/types"
)

type DummyApplication struct {
	mtx   sync.Mutex
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

func (dapp *DummyApplication) Open() types.AppContext {
	dapp.mtx.Lock()
	defer dapp.mtx.Unlock()
	return &DummyAppContext{
		app:   dapp,
		state: dapp.state.Copy(),
	}
}

func (dapp *DummyApplication) commitState(state merkle.Tree) {
	dapp.mtx.Lock()
	defer dapp.mtx.Unlock()
	dapp.state = state.Copy()
}

func (dapp *DummyApplication) getState() merkle.Tree {
	dapp.mtx.Lock()
	defer dapp.mtx.Unlock()
	return dapp.state.Copy()
}

//--------------------------------------------------------------------------------

type DummyAppContext struct {
	app   *DummyApplication
	state merkle.Tree
}

func (dac *DummyAppContext) Echo(message string) string {
	return message
}

func (dac *DummyAppContext) Info() []string {
	return []string{Fmt("size:%v", dac.state.Size())}
}

func (dac *DummyAppContext) SetOption(key string, value string) types.RetCode {
	return 0
}

func (dac *DummyAppContext) AppendTx(tx []byte) ([]types.Event, types.RetCode) {
	dac.state.Set(tx, tx)
	return nil, 0
}

func (dac *DummyAppContext) GetHash() ([]byte, types.RetCode) {
	hash := dac.state.Hash()
	return hash, 0
}

func (dac *DummyAppContext) Commit() types.RetCode {
	dac.app.commitState(dac.state)
	return 0
}

func (dac *DummyAppContext) Rollback() types.RetCode {
	dac.state = dac.app.getState()
	return 0
}

func (dac *DummyAppContext) AddListener(key string) types.RetCode {
	return 0
}

func (dac *DummyAppContext) RemListener(key string) types.RetCode {
	return 0
}

func (dac *DummyAppContext) Close() error {
	return nil
}
