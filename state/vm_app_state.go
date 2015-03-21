package state

import (
	"bytes"
	"sort"

	ac "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/merkle"
	"github.com/tendermint/tendermint/vm"
	"github.com/tendermint/tendermint/vm/sha3"
)

// Converts account.Account to account.VmAccount struct.
func toVMAccount(acc *ac.Account) *ac.VmAccount {
	return &ac.VmAccount{
		Address:     vm.BytesToWord(acc.Address),
		Balance:     acc.Balance,
		Code:        acc.Code, // This is crazy.
		Nonce:       uint64(acc.Sequence),
		StorageRoot: vm.BytesToWord(acc.StorageRoot),
		PubKey:      acc.PubKey,
	}
}

// Converts account.VmAccount to account.Account struct.
func toStateAccount(acc *ac.VmAccount) *ac.Account {
	return &ac.Account{
		Address:     acc.Address.Address(),
		Balance:     acc.Balance,
		Code:        acc.Code,
		Sequence:    uint(acc.Nonce),
		StorageRoot: acc.StorageRoot.Bytes(),
		PubKey:      acc.PubKey,
	}
}

//-----------------------------------------------------------------------------

type AccountInfo struct {
	account vm.Account
	deleted bool
}

type VMAppState struct {
	state *State

	accounts map[string]AccountInfo
	storage  map[string]vm.Word
	logs     []*vm.Log
}

func NewVMAppState(state *State) *VMAppState {
	return &VMAppState{
		state:    state,
		accounts: make(map[string]AccountInfo),
		storage:  make(map[string]vm.Word),
		logs:     make([]*vm.Log, 0),
	}
}

func unpack(accInfo AccountInfo) (*ac.VmAccount, bool) {
	if accInfo.account == nil {
		return nil, accInfo.deleted
	}
	return accInfo.account.(*ac.VmAccount), accInfo.deleted
}

func (vas *VMAppState) GetAccount(addr vm.Word) (vm.Account, error) {
	account, deleted := unpack(vas.accounts[addr.String()])
	if deleted {
		return nil, Errorf("Account was deleted: %X", addr)
	} else if account != nil {
		return account, nil
	} else {
		acc := vas.state.GetAccount(addr.Address())
		if acc == nil {
			return nil, Errorf("Invalid account addr: %X", addr)
		}
		return toVMAccount(acc), nil
	}
}

func (vas *VMAppState) UpdateAccount(account vm.Account) error {
	accountInfo, ok := vas.accounts[account.GetAddress().String()]
	if !ok {
		vas.accounts[account.GetAddress().String()] = AccountInfo{account, false}
		return nil
	}
	vmAccount, deleted := unpack(accountInfo)
	if deleted {
		return Errorf("Account was deleted: %X", vmAccount.Address)
	} else {
		vas.accounts[vmAccount.Address.String()] = AccountInfo{vmAccount, false}
		return nil
	}
}

func (vas *VMAppState) DeleteAccount(account vm.Account) error {
	accountInfo, ok := vas.accounts[account.GetAddress().String()]
	if !ok {
		vas.accounts[account.GetAddress().String()] = AccountInfo{account, true}
		return nil
	}
	vmAccount, deleted := unpack(accountInfo)
	if deleted {
		return Errorf("Account was already deleted: %X", vmAccount.Address)
	} else {
		vas.accounts[vmAccount.Address.String()] = AccountInfo{vmAccount, true}
		return nil
	}
}

// Creates a 20 byte address and bumps the creator's nonce.
func (vas *VMAppState) CreateAccount(creator vm.Account) (vm.Account, error) {

	// Generate an address
	nonce := creator.GetNonce()
	creator.SetNonce(nonce + 1)
	temp := make([]byte, 32+8)
	addr := creator.GetAddress()
	copy(temp, addr[:])
	vm.PutUint64(temp[32:], nonce)
	addr = vm.RightPadWord(sha3.Sha3(temp)[:20])

	// Create account from address.
	account, deleted := unpack(vas.accounts[addr.String()])
	if deleted || account == nil {
		account = &ac.VmAccount{
			Address:     addr,
			Balance:     0,
			Code:        nil,
			Nonce:       0,
			StorageRoot: vm.Zero,
			PubKey:      ac.PubKeyNil{},
		}
		vas.accounts[addr.String()] = AccountInfo{account, false}
		return account, nil
	} else {
		panic(Fmt("Could not create account, address already exists: %X", addr))
		// return nil, Errorf("Account already exists: %X", addr)
	}
}

func (vas *VMAppState) GetStorage(addr vm.Word, key vm.Word) (vm.Word, error) {
	account, deleted := unpack(vas.accounts[addr.String()])
	if account == nil {
		return vm.Zero, Errorf("Invalid account addr: %X", addr)
	} else if deleted {
		return vm.Zero, Errorf("Account was deleted: %X", addr)
	}

	value, ok := vas.storage[addr.String()+key.String()]
	if ok {
		return value, nil
	} else {
		return vm.Zero, nil
	}
}

// NOTE: Set value to zero to delete from the trie.
func (vas *VMAppState) SetStorage(addr vm.Word, key vm.Word, value vm.Word) (bool, error) {
	account, deleted := unpack(vas.accounts[addr.String()])
	if account == nil {
		return false, Errorf("Invalid account addr: %X", addr)
	} else if deleted {
		return false, Errorf("Account was deleted: %X", addr)
	}

	_, ok := vas.storage[addr.String()+key.String()]
	vas.storage[addr.String()+key.String()] = value
	return ok, nil
}

// CONTRACT the updates are in deterministic order.
func (vas *VMAppState) Sync() {

	// Determine order for accounts
	addrStrs := []string{}
	for addrStr := range vas.accounts {
		addrStrs = append(addrStrs, addrStr)
	}
	sort.Strings(addrStrs)

	// Update or delete accounts.
	for _, addrStr := range addrStrs {
		account, deleted := unpack(vas.accounts[addrStr])
		if deleted {
			removed := vas.state.RemoveAccount(account.Address.Address())
			if !removed {
				panic(Fmt("Could not remove account to be deleted: %X", account.Address))
			}
		} else {
			if account == nil {
				panic(Fmt("Account should not be nil for addr: %X", account.Address))
			}
			vas.state.UpdateAccount(toStateAccount(account))
		}
	}

	// Determine order for storage updates
	// The address comes first so it'll be grouped.
	storageKeyStrs := []string{}
	for keyStr := range vas.storage {
		storageKeyStrs = append(storageKeyStrs, keyStr)
	}
	sort.Strings(storageKeyStrs)

	// Update storage for all account/key.
	storage := merkle.NewIAVLTree(
		binary.BasicCodec, // TODO change
		binary.BasicCodec, // TODO change
		1024,              // TODO change.
		vas.state.DB,
	)
	var currentAccount *ac.VmAccount
	var deleted bool
	for _, storageKey := range storageKeyStrs {
		value := vas.storage[storageKey]
		addrKeyBytes := []byte(storageKey)
		addr := addrKeyBytes[:32]
		key := addrKeyBytes[32:]
		if currentAccount == nil || !bytes.Equal(currentAccount.Address[:], addr) {
			currentAccount, deleted = unpack(vas.accounts[string(addr)])
			if deleted {
				continue
			}
			storageRoot := currentAccount.StorageRoot
			storage.Load(storageRoot.Bytes())
		}
		if value.IsZero() {
			_, removed := storage.Remove(key)
			if !removed {
				panic(Fmt("Storage could not be removed for addr: %X @ %X", addr, key))
			}
		} else {
			storage.Set(key, value)
		}
	}

	// TODO support logs, add them to the state somehow.
}

func (vas *VMAppState) AddLog(log *vm.Log) {
	vas.logs = append(vas.logs, log)
}
