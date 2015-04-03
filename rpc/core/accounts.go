package core

import (
	"fmt"
	"github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
)

func GenPrivAccount() (*ResponseGenPrivAccount, error) {
	return &ResponseGenPrivAccount{account.GenPrivAccount()}, nil
}

func GetAccount(address []byte) (*ResponseGetAccount, error) {
	cache := mempoolReactor.Mempool.GetCache()
	return &ResponseGetAccount{cache.GetAccount(address)}, nil
}

func GetStorage(address, storage []byte) (*ResponseGetStorage, error) {
	state := consensusState.GetState()
	account := state.GetAccount(address)
	if account == nil {
		return nil, fmt.Errorf("Unknown address: %X", address)
	}
	storageRoot := account.StorageRoot
	storageTree := state.LoadStorage(storageRoot)

	_, value := storageTree.Get(RightPadWord256(storage).Bytes())
	if value == nil {
		return &ResponseGetStorage{storage, nil}, nil
	}
	return &ResponseGetStorage{storage, value.([]byte)}, nil
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
