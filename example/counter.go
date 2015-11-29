package example

import (
	"encoding/binary"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/types"
)

type CounterApplication struct {
	hashCount     int
	lastHashCount int

	txCount     int
	lastTxCount int

	commitCount int
}

func NewCounterApplication() *CounterApplication {
	return &CounterApplication{}
}

func (dapp *CounterApplication) Echo(message string) string {
	return message
}

func (dapp *CounterApplication) Info() []string {
	return []string{Fmt("hash, tx, commit counts:%d, %d, %d", dapp.hashCount, dapp.txCount, dapp.commitCount)}
}

func (dapp *CounterApplication) SetOption(key string, value string) types.RetCode {
	return 0
}

func (dapp *CounterApplication) AppendTx(tx []byte) ([]types.Event, types.RetCode) {
	dapp.txCount += 1
	return nil, 0
}

func (dapp *CounterApplication) GetHash() ([]byte, types.RetCode) {
	hash := make([]byte, 32)
	binary.PutVarint(hash, int64(dapp.hashCount))
	dapp.hashCount += 1
	return hash, 0
}

func (dapp *CounterApplication) Commit() types.RetCode {
	dapp.lastHashCount = dapp.hashCount
	dapp.lastTxCount = dapp.txCount
	dapp.commitCount += 1
	return 0
}

func (dapp *CounterApplication) Rollback() types.RetCode {
	dapp.hashCount = dapp.lastHashCount
	dapp.txCount = dapp.lastTxCount
	return 0
}

func (dapp *CounterApplication) AddListener(key string) types.RetCode {
	return 0
}

func (dapp *CounterApplication) RemListener(key string) types.RetCode {
	return 0
}
