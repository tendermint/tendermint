package core

import (
	"fmt"
	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

func GenPrivAccount() (*acm.PrivAccount, error) {
	return acm.GenPrivAccount(), nil
}

// If the account is not known, returns nil, nil.
func GetAccount(address []byte) (*acm.Account, error) {
	cache := mempoolReactor.Mempool.GetCache()
	account := cache.GetAccount(address)
	if account == nil {
		return nil, nil
	}
	return account, nil
}

func GetStorage(address, key []byte) (*ctypes.ResponseGetStorage, error) {
	state := consensusState.GetState()
	account := state.GetAccount(address)
	if account == nil {
		return nil, fmt.Errorf("UnknownAddress: %X", address)
	}
	storageRoot := account.StorageRoot
	storageTree := state.LoadStorage(storageRoot)

	_, value := storageTree.Get(LeftPadWord256(key).Bytes())
	if value == nil {
		return &ctypes.ResponseGetStorage{key, nil}, nil
	}
	return &ctypes.ResponseGetStorage{key, value.([]byte)}, nil
}

func ListAccounts() (*ctypes.ResponseListAccounts, error) {
	var blockHeight int
	var accounts []*acm.Account
	state := consensusState.GetState()
	blockHeight = state.LastBlockHeight
	state.GetAccounts().Iterate(func(key interface{}, value interface{}) bool {
		accounts = append(accounts, value.(*acm.Account))
		return false
	})
	return &ctypes.ResponseListAccounts{blockHeight, accounts}, nil
}

func DumpStorage(address []byte) (*ctypes.ResponseDumpStorage, error) {
	state := consensusState.GetState()
	account := state.GetAccount(address)
	if account == nil {
		return nil, fmt.Errorf("UnknownAddress: %X", address)
	}
	storageRoot := account.StorageRoot
	storageTree := state.LoadStorage(storageRoot)
	storageItems := []ctypes.StorageItem{}
	storageTree.Iterate(func(key interface{}, value interface{}) bool {
		storageItems = append(storageItems, ctypes.StorageItem{
			key.([]byte), value.([]byte)})
		return false
	})
	return &ctypes.ResponseDumpStorage{storageRoot, storageItems}, nil
}
