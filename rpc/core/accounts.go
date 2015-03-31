package core

import (
	"github.com/tendermint/tendermint2/account"
)

//-----------------------------------------------------------------------------

func GenPrivAccount() (*ResponseGenPrivAccount, error) {
	return &ResponseGenPrivAccount{account.GenPrivAccount()}, nil
}

//-----------------------------------------------------------------------------

func GetAccount(address []byte) (*ResponseGetAccount, error) {
	cache := mempoolReactor.Mempool.GetCache()
	return &ResponseGetAccount{cache.GetAccount(address)}, nil
}

//-----------------------------------------------------------------------------

func ListAccounts() (*ResponseListAccounts, error) {
	var blockHeight uint
	var accounts []*account.Account
	state := consensusState.GetState()
	blockHeight = state.LastBlockHeight
	state.GetAccounts().Iterate(func(key interface{}, value interface{}) bool {
		accounts = append(accounts, value.(*account.Account))
		return false
	})
	return &ResponseListAccounts{blockHeight, accounts}, nil
}
