package vm

import ()

const (
	defaultDataStackCapacity = 10
)

type Account struct {
	Address   Word
	Balance   uint64
	Code      []byte
	Nonce     uint64
	StateRoot Word
}

type Log struct {
	Address Word
	Topics  []Word
	Data    []byte
	Height  uint64
}

type AppState interface {

	// Accounts
	GetAccount(addr Word) (*Account, error)
	UpdateAccount(*Account) error
	DeleteAccount(*Account) error
	CreateAccount(addr Word, balance uint64) (*Account, error)

	// Storage
	GetStorage(Word, Word) (Word, error)
	SetStorage(Word, Word, Word) (bool, error)
	RemoveStorage(Word, Word) error

	// Logs
	AddLog(*Log)
}
