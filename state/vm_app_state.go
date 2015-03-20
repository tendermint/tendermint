package state

import (
	"sort"

	ac "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/merkle"
	"github.com/tendermint/tendermint/vm"
)

// Converts state.Account to vm.Account struct.
func toVMAccount(acc *ac.Account) *vm.Account {
	return &vm.Account{
		Address:     vm.BytesToWord(acc.Address),
		Balance:     acc.Balance,
		Code:        acc.Code, // This is crazy.
		Nonce:       uint64(acc.Sequence),
		StorageRoot: vm.BytesToWord(acc.StorageRoot),
	}
}

// Converts vm.Account to state.Account struct.
func toStateAccount(acc *vm.Account) *ac.Account {
	return &ac.Account{
		Address:     acc.Address.Address(),
		Balance:     acc.Balance,
		Code:        acc.Code,
		Sequence:    uint(acc.Nonce),
		StorageRoot: acc.StorageRoot.Bytes(),
	}
}

//-----------------------------------------------------------------------------

type AccountInfo struct {
	account *vm.Account
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

func unpack(accInfo AccountInfo) (*vm.Account, bool) {
	return accInfo.account, accInfo.deleted
}

// Used to add the origin of the tx to VMAppState.
func (vas *VMAppState) AddAccount(account *vm.Account) error {
	if _, ok := vas.accounts[account.Address.String()]; ok {
		return Errorf("Account already exists: %X", account.Address)
	} else {
		vas.accounts[account.Address.String()] = AccountInfo{account, false}
		return nil
	}
}

func (vas *VMAppState) GetAccount(addr vm.Word) (*vm.Account, error) {
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

func (vas *VMAppState) UpdateAccount(account *vm.Account) error {
	accountInfo, ok := vas.accounts[account.Address.String()]
	if !ok {
		vas.accounts[account.Address.String()] = AccountInfo{account, false}
		return nil
	}
	account, deleted := unpack(accountInfo)
	if deleted {
		return Errorf("Account was deleted: %X", account.Address)
	} else {
		vas.accounts[account.Address.String()] = AccountInfo{account, false}
		return nil
	}
}

func (vas *VMAppState) DeleteAccount(account *vm.Account) error {
	accountInfo, ok := vas.accounts[account.Address.String()]
	if !ok {
		vas.accounts[account.Address.String()] = AccountInfo{account, true}
		return nil
	}
	account, deleted := unpack(accountInfo)
	if deleted {
		return Errorf("Account was already deleted: %X", account.Address)
	} else {
		vas.accounts[account.Address.String()] = AccountInfo{account, true}
		return nil
	}
}

func (vas *VMAppState) CreateAccount(addr vm.Word) (*vm.Account, error) {
	account, deleted := unpack(vas.accounts[addr.String()])
	if deleted || account == nil {
		account = &vm.Account{
			Address:     addr,
			Balance:     0,
			Code:        nil,
			Nonce:       0,
			StorageRoot: vm.Zero,
		}
		vas.accounts[addr.String()] = AccountInfo{account, false}
		return account, nil
	} else {
		return nil, Errorf("Account already exists: %X", addr)
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

	// Update or delete storage items.
	storage := merkle.NewIAVLTree(
		binary.BasicCodec, // TODO change
		binary.BasicCodec, // TODO change
		1024,              // TODO change.
		vas.state.DB,
	)

	for addrKey, value := range vas.storage {
		addrKeyBytes := []byte(addrKey)
		addr := addrKeyBytes[:32]
		key := addrKeyBytes[32:]
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
