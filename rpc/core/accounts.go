package core

import (
	"github.com/tendermint/tendermint/account"
)

//-----------------------------------------------------------------------------

func GenPrivAccount() *account.PrivAccount {
	return account.GenPrivAccount()
}

//-----------------------------------------------------------------------------

func GetAccount(address []byte) *account.Account {
	state := consensusState.GetState()
	return state.GetAccount(address)
}

//-----------------------------------------------------------------------------

func ListAccounts() (uint, []*account.Account) {
	var blockHeight uint
	var accounts []*account.Account
	state := consensusState.GetState()
	blockHeight = state.LastBlockHeight
	state.GetAccounts().Iterate(func(key interface{}, value interface{}) bool {
		accounts = append(accounts, value.(*account.Account))
		return false
	})
	return blockHeight, accounts
}
