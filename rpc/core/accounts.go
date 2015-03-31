package core

import (
	"fmt"
	"github.com/tendermint/tendermint2/account"
	. "github.com/tendermint/tendermint2/common"
)

func GenPrivAccount() (*ResponseGenPrivAccount, error) {
	return &ResponseGenPrivAccount{account.GenPrivAccount()}, nil
}

func GetAccount(addr []byte) (*ResponseGetAccount, error) {
	cache := mempoolReactor.Mempool.GetCache()
	return &ResponseGetAccount{cache.GetAccount(addr)}, nil
}

func GetStorage(address, slot []byte) (*ResponseGetStorage, error) {
	state := consensusState.GetState()
	account := state.GetAccount(address)
	if account == nil {
		return nil, fmt.Errorf("Unknown address: %X", address)
	}
	storageRoot := account.StorageRoot
	storage := state.LoadStorage(storageRoot)

	_, value := storage.Get(RightPadWord256(slot).Bytes())
	if value == nil {
		return &ResponseGetStorage{slot, nil}, nil
	}
	return &ResponseGetStorage{slot, value.([]byte)}, nil
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

func GetStorage(address, storage []byte) (*ResponseGetStorage, error) {
	cache := mempoolReactor.Mempool.GetCache()
	addr, slot := RightPadWord256(address), RightPadWord256(storage)
	value := cache.GetStorage(addr, slot)
	fmt.Printf("STORAGE: %x, %x, %x\n", addr, slot, value)
	return &ResponseGetStorage{storage, value.Bytes()}, nil
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
