package example

import (
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-merkle"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tmsp/types"
)

type DummyApplication struct {
	state           merkle.Tree
	lastCommitState merkle.Tree
}

func NewDummyApplication() *DummyApplication {
	state := merkle.NewIAVLTree(
		wire.BasicCodec,
		wire.BasicCodec,
		0,
		nil,
	)
	return &DummyApplication{
		state:           state,
		lastCommitState: state,
	}
}

func (dapp *DummyApplication) Echo(message string) string {
	return message
}

func (dapp *DummyApplication) Info() []string {
	return []string{Fmt("size:%v", dapp.state.Size())}
}

func (dapp *DummyApplication) SetOption(key string, value string) types.RetCode {
	return 0
}

func (dapp *DummyApplication) AppendTx(tx []byte) ([]types.Event, types.RetCode) {
	dapp.state.Set(tx, tx)
	return nil, 0
}

func (dapp *DummyApplication) GetHash() ([]byte, types.RetCode) {
	hash := dapp.state.Hash()
	return hash, 0
}

func (dapp *DummyApplication) Commit() types.RetCode {
	dapp.lastCommitState = dapp.state.Copy()
	return 0
}

func (dapp *DummyApplication) Rollback() types.RetCode {
	dapp.state = dapp.lastCommitState.Copy()
	return 0
}

func (dapp *DummyApplication) AddListener(key string) types.RetCode {
	return 0
}

func (dapp *DummyApplication) RemListener(key string) types.RetCode {
	return 0
}
