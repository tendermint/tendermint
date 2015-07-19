package state

import (
	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/vm"
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
