package state

import (
	"bytes"
	"sort"

	ac "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	dbm "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/merkle"
)

func makeStorage(db dbm.DB, root []byte) merkle.Tree {
	storage := merkle.NewIAVLTree(
		binary.BasicCodec,
		binary.BasicCodec,
		1024,
		db,
	)
	storage.Load(root)
	return storage
}

type BlockCache struct {
	db       dbm.DB
	backend  *State
	accounts map[string]accountInfo
	storages map[Tuple256]storageInfo
}

func NewBlockCache(backend *State) *BlockCache {
	return &BlockCache{
		db:       backend.DB,
		backend:  backend,
		accounts: make(map[string]accountInfo),
		storages: make(map[Tuple256]storageInfo),
	}
}

func (cache *BlockCache) State() *State {
	return cache.backend
}

//-------------------------------------
// BlockCache.account

func (cache *BlockCache) GetAccount(addr []byte) *ac.Account {
	acc, _, removed, _ := cache.accounts[string(addr)].unpack()
	if removed {
		return nil
	} else if acc != nil {
		return acc
	} else {
		acc = cache.backend.GetAccount(addr)
		cache.accounts[string(addr)] = accountInfo{acc, nil, false, false}
		return acc
	}
}

func (cache *BlockCache) UpdateAccount(acc *ac.Account) {
	addr := acc.Address
	// SANITY CHECK
	_, storage, removed, _ := cache.accounts[string(addr)].unpack()
	if removed {
		panic("UpdateAccount on a removed account")
	}
	// SANITY CHECK END
	cache.accounts[string(addr)] = accountInfo{acc, storage, false, true}
}

func (cache *BlockCache) RemoveAccount(addr []byte) {
	// SANITY CHECK
	_, _, removed, _ := cache.accounts[string(addr)].unpack()
	if removed {
		panic("RemoveAccount on a removed account")
	}
	// SANITY CHECK END
	cache.accounts[string(addr)] = accountInfo{nil, nil, true, false}
}

// BlockCache.account
//-------------------------------------
// BlockCache.storage

func (cache *BlockCache) GetStorage(addr Word256, key Word256) (value Word256) {
	// Check cache
	info, ok := cache.storages[Tuple256{addr, key}]
	if ok {
		return info.value
	}

	// Get or load storage
	acc, storage, removed, dirty := cache.accounts[string(addr.Postfix(20))].unpack()
	if removed {
		panic("GetStorage() on removed account")
	}
	if acc != nil && storage == nil {
		storage = makeStorage(cache.db, acc.StorageRoot)
		cache.accounts[string(addr.Postfix(20))] = accountInfo{acc, storage, false, dirty}
	} else if acc == nil {
		return Zero256
	}

	// Load and set cache
	_, val_ := storage.Get(key.Bytes())
	value = Zero256
	if val_ != nil {
		value = RightPadWord256(val_.([]byte))
	}
	cache.storages[Tuple256{addr, key}] = storageInfo{value, false}
	return value
}

// NOTE: Set value to zero to removed from the trie.
func (cache *BlockCache) SetStorage(addr Word256, key Word256, value Word256) {
	_, _, removed, _ := cache.accounts[string(addr.Postfix(20))].unpack()
	if removed {
		panic("SetStorage() on a removed account")
	}
	cache.storages[Tuple256{addr, key}] = storageInfo{value, true}
}

// BlockCache.storage
//-------------------------------------

// CONTRACT the updates are in deterministic order.
func (cache *BlockCache) Sync() {

	// Determine order for storage updates
	// The address comes first so it'll be grouped.
	storageKeys := make([]Tuple256, 0, len(cache.storages))
	for keyTuple := range cache.storages {
		storageKeys = append(storageKeys, keyTuple)
	}
	Tuple256Slice(storageKeys).Sort()

	// Update storage for all account/key.
	// Later we'll iterate over all the users and save storage + update storage root.
	var (
		curAddr       Word256
		curAcc        *ac.Account
		curAccRemoved bool
		curStorage    merkle.Tree
	)
	for _, storageKey := range storageKeys {
		addr, key := Tuple256Split(storageKey)
		if addr != curAddr || curAcc == nil {
			acc, storage, removed, _ := cache.accounts[string(addr.Postfix(20))].unpack()
			if storage == nil {
				storage = makeStorage(cache.db, acc.StorageRoot)
			}
			curAddr = addr
			curAcc = acc
			curAccRemoved = removed
			curStorage = storage
		}
		if curAccRemoved {
			continue
		}
		value, dirty := cache.storages[storageKey].unpack()
		if !dirty {
			continue
		}
		if value.IsZero() {
			curStorage.Remove(key.Bytes())
		} else {
			curStorage.Set(key.Bytes(), value.Bytes())
			cache.accounts[string(addr.Postfix(20))] = accountInfo{curAcc, curStorage, false, true}
		}
	}

	// Determine order for accounts
	addrStrs := []string{}
	for addrStr := range cache.accounts {
		addrStrs = append(addrStrs, addrStr)
	}
	sort.Strings(addrStrs)

	// Update or delete accounts.
	for _, addrStr := range addrStrs {
		acc, storage, removed, dirty := cache.accounts[addrStr].unpack()
		if removed {
			removed := cache.backend.RemoveAccount(acc.Address)
			if !removed {
				panic(Fmt("Could not remove account to be removed: %X", acc.Address))
			}
		} else {
			if acc == nil {
				continue
			}
			if storage != nil {
				newStorageRoot := storage.Save()
				if !bytes.Equal(newStorageRoot, acc.StorageRoot) {
					acc.StorageRoot = newStorageRoot
					dirty = true
				}
			}
			if dirty {
				cache.backend.UpdateAccount(acc)
			}
		}
	}

}

//-----------------------------------------------------------------------------

type accountInfo struct {
	account *ac.Account
	storage merkle.Tree
	removed bool
	dirty   bool
}

func (accInfo accountInfo) unpack() (*ac.Account, merkle.Tree, bool, bool) {
	return accInfo.account, accInfo.storage, accInfo.removed, accInfo.dirty
}

type storageInfo struct {
	value Word256
	dirty bool
}

func (stjInfo storageInfo) unpack() (Word256, bool) {
	return stjInfo.value, stjInfo.dirty
}
