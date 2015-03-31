package core

import (
	"fmt"
	"github.com/tendermint/tendermint2/account"
)

func GenPrivAccount() (*ResponseGenPrivAccount, error) {
	return &ResponseGenPrivAccount{account.GenPrivAccount()}, nil
}

func GetAccount(addr []byte) (*ResponseGetAccount, error) {
	cache := mempoolReactor.Mempool.GetCache()
	return &ResponseGetAccount{cache.GetAccount(addr)}, nil
}

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

func DumpStorage(addr []byte) (*ResponseDumpStorage, error) {
	state := consensusState.GetState()
	account := state.GetAccount(addr)
	if account == nil {
		return nil, fmt.Errorf("Unknown address: %X", addr)
	}
	storageRoot := account.StorageRoot
	storage := state.LoadStorage(storageRoot)
	storageItems := []StorageItem{}
	storage.Iterate(func(key interface{}, value interface{}) bool {
		storageItems = append(storageItems, StorageItem{
			key.([]byte), value.([]byte)})
		return false
	})
	return &ResponseDumpStorage{storageRoot, storageItems}, nil
}
