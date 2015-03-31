package state

import (
	ac "github.com/tendermint/tendermint2/account"
	. "github.com/tendermint/tendermint2/common"
	"github.com/tendermint/tendermint2/vm"
)

type AccountGetter interface {
	GetAccount(addr []byte) *ac.Account
}

type VMAccountState interface {
	GetAccount(addr Word256) *vm.Account
	UpdateAccount(acc *vm.Account)
	RemoveAccount(acc *vm.Account)
	CreateAccount(creator *vm.Account) *vm.Account
}
