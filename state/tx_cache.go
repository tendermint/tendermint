package state

import (
	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	ptypes "github.com/tendermint/tendermint/permission/types" // for GlobalPermissionAddress ...
	"github.com/tendermint/tendermint/vm"
	"github.com/tendermint/tendermint/vm/sha3"
)

type TxCache struct {
	backend  *BlockCache
	accounts map[Word256]vmAccountInfo
	storages map[Tuple256]Word256
	logs     []*vm.Log
}

func NewTxCache(backend *BlockCache) *TxCache {
	return &TxCache{
		backend:  backend,
		accounts: make(map[Word256]vmAccountInfo),
		storages: make(map[Tuple256]Word256),
		logs:     make([]*vm.Log, 0),
	}
}

//-------------------------------------
// TxCache.account

func (cache *TxCache) GetAccount(addr Word256) *vm.Account {
	acc, removed := vmUnpack(cache.accounts[addr])
	if removed {
		return nil
	} else if acc == nil {
		acc2 := cache.backend.GetAccount(addr.Postfix(20))
		if acc2 != nil {
			return toVMAccount(acc2)
		}
	}
	return acc
}

func (cache *TxCache) UpdateAccount(acc *vm.Account) {
	addr := acc.Address
	_, removed := vmUnpack(cache.accounts[addr])
	if removed {
		PanicSanity("UpdateAccount on a removed account")
	}
	cache.accounts[addr] = vmAccountInfo{acc, false}
}

func (cache *TxCache) RemoveAccount(acc *vm.Account) {
	addr := acc.Address
	_, removed := vmUnpack(cache.accounts[addr])
	if removed {
		PanicSanity("RemoveAccount on a removed account")
	}
	cache.accounts[addr] = vmAccountInfo{acc, true}
}

// Creates a 20 byte address and bumps the creator's nonce.
func (cache *TxCache) CreateAccount(creator *vm.Account) *vm.Account {

	// Generate an address
	nonce := creator.Nonce
	creator.Nonce += 1

	addr := LeftPadWord256(NewContractAddress(creator.Address.Postfix(20), int(nonce)))

	// Create account from address.
	account, removed := vmUnpack(cache.accounts[addr])
	if removed || account == nil {
		account = &vm.Account{
			Address:     addr,
			Balance:     0,
			Code:        nil,
			Nonce:       0,
			StorageRoot: Zero256,
			Permissions: cache.GetAccount(ptypes.GlobalPermissionsAddress256).Permissions,
			Other:       nil,
		}
		cache.accounts[addr] = vmAccountInfo{account, false}
		return account
	} else {
		// either we've messed up nonce handling, or sha3 is broken
		PanicSanity(Fmt("Could not create account, address already exists: %X", addr))
		return nil
	}
}

// TxCache.account
//-------------------------------------
// TxCache.storage

func (cache *TxCache) GetStorage(addr Word256, key Word256) Word256 {
	// Check cache
	value, ok := cache.storages[Tuple256{addr, key}]
	if ok {
		return value
	}

	// Load from backend
	return cache.backend.GetStorage(addr, key)
}

// NOTE: Set value to zero to removed from the trie.
func (cache *TxCache) SetStorage(addr Word256, key Word256, value Word256) {
	_, removed := vmUnpack(cache.accounts[addr])
	if removed {
		PanicSanity("SetStorage() on a removed account")
	}
	cache.storages[Tuple256{addr, key}] = value
}

// TxCache.storage
//-------------------------------------

// These updates do not have to be in deterministic order,
// the backend is responsible for ordering updates.
func (cache *TxCache) Sync() {

	// Remove or update storage
	for addrKey, value := range cache.storages {
		addr, key := Tuple256Split(addrKey)
		cache.backend.SetStorage(addr, key, value)
	}

	// Remove or update accounts
	for addr, accInfo := range cache.accounts {
		acc, removed := vmUnpack(accInfo)
		if removed {
			cache.backend.RemoveAccount(addr.Postfix(20))
		} else {
			cache.backend.UpdateAccount(toStateAccount(acc))
		}
	}

	// TODO support logs, add them to the cache somehow.
}

func (cache *TxCache) AddLog(log *vm.Log) {
	cache.logs = append(cache.logs, log)
}

//-----------------------------------------------------------------------------

// Convenience function to return address of new contract
func NewContractAddress(caller []byte, nonce int) []byte {
	temp := make([]byte, 32+8)
	copy(temp, caller)
	PutInt64BE(temp[32:], int64(nonce))
	return sha3.Sha3(temp)[:20]
}

// Converts backend.Account to vm.Account struct.
func toVMAccount(acc *acm.Account) *vm.Account {
	return &vm.Account{
		Address:     LeftPadWord256(acc.Address),
		Balance:     acc.Balance,
		Code:        acc.Code, // This is crazy.
		Nonce:       int64(acc.Sequence),
		StorageRoot: LeftPadWord256(acc.StorageRoot),
		Permissions: acc.Permissions, // Copy
		Other:       acc.PubKey,
	}
}

// Converts vm.Account to backend.Account struct.
func toStateAccount(acc *vm.Account) *acm.Account {
	pubKey, ok := acc.Other.(acm.PubKey)
	if !ok {
		pubKey = nil
	}

	var storageRoot []byte
	if acc.StorageRoot.IsZero() {
		storageRoot = nil
	} else {
		storageRoot = acc.StorageRoot.Bytes()
	}
	return &acm.Account{
		Address:     acc.Address.Postfix(20),
		PubKey:      pubKey,
		Balance:     acc.Balance,
		Code:        acc.Code,
		Sequence:    int(acc.Nonce),
		StorageRoot: storageRoot,
		Permissions: acc.Permissions, // Copy
	}
}

type vmAccountInfo struct {
	account *vm.Account
	removed bool
}

func vmUnpack(accInfo vmAccountInfo) (*vm.Account, bool) {
	return accInfo.account, accInfo.removed
}
