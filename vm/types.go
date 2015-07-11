package vm

import (
	"encoding/hex"
	. "github.com/tendermint/tendermint/common"
	ptypes "github.com/tendermint/tendermint/permission/types"
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

	Permissions ptypes.AccountPermissions
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

type SolLog struct {
	Address string
	Topics  []string
	Data    string
	Height  int64
}

func toSolLog(log *Log) *SolLog{
	ts := make([]string, len(log.Topics))
	for i := 0; i < len(log.Topics); i++ {
		ts[i] = hex.EncodeToString(log.Topics[i].Bytes())
	}
	return &SolLog{
		hex.EncodeToString(log.Address.Bytes()),
		ts,
		hex.EncodeToString(log.Data),
		log.Height,
	}
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
