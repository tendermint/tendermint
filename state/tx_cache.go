package state

import (
	ac "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
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
	} else {
		return acc
	}
}

func (cache *TxCache) UpdateAccount(acc *vm.Account) {
	addr := acc.Address
	// SANITY CHECK
	_, removed := vmUnpack(cache.accounts[addr])
	if removed {
		panic("UpdateAccount on a removed account")
	}
	// SANITY CHECK END
	cache.accounts[addr] = vmAccountInfo{acc, false}
}

func (cache *TxCache) RemoveAccount(acc *vm.Account) {
	addr := acc.Address
	// SANITY CHECK
	_, removed := vmUnpack(cache.accounts[addr])
	if removed {
		panic("RemoveAccount on a removed account")
	}
	// SANITY CHECK END
	cache.accounts[addr] = vmAccountInfo{acc, true}
}

// Creates a 20 byte address and bumps the creator's nonce.
func (cache *TxCache) CreateAccount(creator *vm.Account) *vm.Account {

	// Generate an address
	nonce := creator.Nonce
	creator.Nonce += 1

	addr := RightPadWord256(NewContractAddress(creator.Address.Prefix(20), nonce))

	// Create account from address.
	account, removed := vmUnpack(cache.accounts[addr])
	if removed || account == nil {
		account = &vm.Account{
			Address:     addr,
			Balance:     0,
			Code:        nil,
			Nonce:       0,
			StorageRoot: Zero256,
		}
		cache.accounts[addr] = vmAccountInfo{account, false}
		return account
	} else {
		panic(Fmt("Could not create account, address already exists: %X", addr))
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
		panic("SetStorage() on a removed account")
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
			cache.backend.RemoveAccount(addr.Prefix(20))
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
func NewContractAddress(caller []byte, nonce uint64) []byte {
	temp := make([]byte, 32+8)
	copy(temp, caller)
	PutUint64(temp[32:], nonce)
	return sha3.Sha3(temp)[:20]
}

// Converts backend.Account to vm.Account struct.
func toVMAccount(acc *ac.Account) *vm.Account {
	return &vm.Account{
		Address:     RightPadWord256(acc.Address),
		Balance:     acc.Balance,
		Code:        acc.Code, // This is crazy.
		Nonce:       uint64(acc.Sequence),
		StorageRoot: RightPadWord256(acc.StorageRoot),
		Other:       acc.PubKey,
	}
}

// Converts vm.Account to backend.Account struct.
func toStateAccount(acc *vm.Account) *ac.Account {
	pubKey, ok := acc.Other.(ac.PubKey)
	if !ok {
		pubKey = nil
	}
	var storageRoot []byte
	if acc.StorageRoot.IsZero() {
		storageRoot = nil
	} else {
		storageRoot = acc.StorageRoot.Bytes()
	}
	return &ac.Account{
		Address:     acc.Address.Prefix(20),
		PubKey:      pubKey,
		Balance:     acc.Balance,
		Code:        acc.Code,
		Sequence:    uint(acc.Nonce),
		StorageRoot: storageRoot,
	}
}

type vmAccountInfo struct {
	account *vm.Account
	removed bool
}

func vmUnpack(accInfo vmAccountInfo) (*vm.Account, bool) {
	return accInfo.account, accInfo.removed
}
