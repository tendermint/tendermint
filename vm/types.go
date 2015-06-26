package vm

import (
	. "github.com/tendermint/tendermint/common"
)

const (
	defaultDataStackCapacity = 10
)

type Account struct {
	Address     Word256
	Balance     int64
	Code        []byte
	Nonce       int64
	StorageRoot Word256
	Other       interface{} // For holding all other data.
}

func (acc *Account) String() string {
	return Fmt("VMAccount{%X B:%v C:%X N:%v S:%X}",
		acc.Address, acc.Balance, acc.Code, acc.Nonce, acc.StorageRoot)
}

type Log struct {
	Address Word256
	Topics  []Word256
	Data    []byte
	Height  int64
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
