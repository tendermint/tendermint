package state

import (
	acm "github.com/eris-ltd/tendermint/account"
	. "github.com/eris-ltd/tendermint/common"
	"github.com/eris-ltd/tendermint/vm"
)

type AccountGetter interface {
	GetAccount(addr []byte) *acm.Account
}

type VMAccountState interface {
	GetAccount(addr Word256) *vm.Account
	UpdateAccount(acc *vm.Account)
	RemoveAccount(acc *vm.Account)
	CreateAccount(creator *vm.Account) *vm.Account
}
