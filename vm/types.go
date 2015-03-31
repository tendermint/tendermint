package vm

import (
	. "github.com/tendermint/tendermint2/common"
)

const (
	defaultDataStackCapacity = 10
)

type Account struct {
	Address     Word256
	Balance     uint64
	Code        []byte
	Nonce       uint64
	StorageRoot Word256
	Other       interface{} // For holding all other data.
}

type Log struct {
	Address Word256
	Topics  []Word256
	Data    []byte
	Height  uint64
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
	BlockHeight uint64
	BlockHash   Word256
	BlockTime   int64
	GasLimit    uint64
}
