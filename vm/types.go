package vm

import (
	. "github.com/tendermint/tendermint/common"
	ptypes "github.com/tendermint/tendermint/permission/types"
)

const (
	defaultDataStackCapacity = 10
)

type Account struct {
	Address Word256
	Balance int64
	Code    []byte
	Nonce   int64
	Other   interface{} // For holding all other data.

	Permissions ptypes.AccountPermissions
}

func (acc *Account) String() string {
	if acc == nil {
		return "nil-VMAccount"
	}
	return Fmt("VMAccount{%X B:%v C:%X N:%v}",
		acc.Address, acc.Balance, acc.Code, acc.Nonce)
}

// NOTE: This is serialized as an event from vm/vm.
// See: EventStringLogEvent
type Log struct {
	Address Word256   `json:"address"`
	Topics  []Word256 `json:"topics"`
	Data    []byte    `json:"data"`
	Height  int64     `json:"height"`
}

type AppState interface {

	// Accounts
	GetAccount(addr Word256) *Account
	UpdateAccount(*Account)
	RemoveAccount(*Account)
	CreateAccount(*Account) *Account

	// Storage
	GetStorage(Word256, Word256) Word256
	SetStorage(Word256, Word256, Word256) // Setting to Zero is deleting.

	// Logs
	AddLog(*Log)
}

type Params struct {
	BlockHeight int64
	BlockHash   Word256
	BlockTime   int64
	GasLimit    int64
}
