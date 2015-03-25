package state

import (
	"bytes"
	"fmt"
	"sort"

	ac "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/merkle"
	"github.com/tendermint/tendermint/vm"
	"github.com/tendermint/tendermint/vm/sha3"
)

// Converts state.Account to vm.Account struct.
func toVMAccount(acc *ac.Account) *vm.Account {
	return &vm.Account{
		Address:     vm.BytesToWord(acc.Address),
		Balance:     acc.Balance,
		Code:        acc.Code, // This is crazy.
		Nonce:       uint64(acc.Sequence),
		StorageRoot: vm.BytesToWord(acc.StorageRoot),
		Other:       acc.PubKey,
	}
}

// Converts vm.Account to state.Account struct.
func toStateAccount(acc *vm.Account) *ac.Account {
	pubKey, ok := acc.Other.(ac.PubKey)
	if !ok {
		pubKey = ac.PubKeyNil{}
	}
	var storageRoot []byte
	if acc.StorageRoot.IsZero() {
		storageRoot = nil
	} else {
		storageRoot = acc.StorageRoot.Bytes()
	}
	return &ac.Account{
		Address:     acc.Address.Address(),
		PubKey:      pubKey,
		Balance:     acc.Balance,
		Code:        acc.Code,
		Sequence:    uint(acc.Nonce),
		StorageRoot: storageRoot,
	}
}

//-----------------------------------------------------------------------------

type AccountInfo struct {
	account *vm.Account
	deleted bool
}

type BackendState interface {
	GetAccount(vm.Word) (*vm.Account, error)
	GetStorage(vm.Word, vm.Word) (vm.Word, error)
	UpdateAccount(*vm.Account) error
	RemoveAccount(*vm.Account) error
}

// A cache for storage during block and/or tx processing
type TransState struct {
	// backend state we sync to or fetch from
	state BackendState

	accounts map[string]AccountInfo
	storage  map[string]vm.Word
	logs     []*vm.Log
}

func NewTransState(state BackendState) *TransState {
	return &TransState{
		state:    state,
		accounts: make(map[string]AccountInfo),
		storage:  make(map[string]vm.Word),
		logs:     make([]*vm.Log, 0),
	}
}

func unpack(accInfo AccountInfo) (*vm.Account, bool) {
	return accInfo.account, accInfo.deleted
}

func (vas *TransState) GetAccount(addr vm.Word) (*vm.Account, error) {
	account, deleted := unpack(vas.accounts[addr.String()])
	if deleted {
		return nil, Errorf("Account was deleted: %X", addr)
	} else if account != nil {
		return account, nil
	} else {
		acc, err := vas.state.GetAccount(addr)
		if err != nil {
			return nil, Errorf("Invalid account addr: %X. %s", addr, err.Error())
		}
		return acc, nil
	}
}

func (vas *TransState) UpdateAccount(account *vm.Account) error {
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

func (vas *TransState) RemoveAccount(account *vm.Account) error {
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

// Creates a 20 byte address and bumps the creator's nonce.
func (vas *TransState) CreateAccount(creator *vm.Account) (*vm.Account, error) {

	// Generate an address
	nonce := creator.Nonce
	creator.Nonce += 1

	addr := vm.RightPadWord(NewContractAddress(creator.Address.Address(), nonce))

	// Create account from address.
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
		panic(Fmt("Could not create account, address already exists: %X", addr))
		// return nil, Errorf("Account already exists: %X", addr)
	}
}

func (vas *TransState) GetStorage(addr vm.Word, key vm.Word) (vm.Word, error) {
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
		res, err := vas.state.GetStorage(addr, key)
		if err != nil {
			return vm.Zero, err
		}
		return res, nil
	}
}

// NOTE: Set value to zero to delete from the trie.
func (vas *TransState) SetStorage(addr vm.Word, key vm.Word, value vm.Word) (bool, error) {
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

// Sync 'vas' to 'vas.state'. If 'vas.state' is the 'blockState',
// we should do a hashmap merge. Otherwise, we need the
// keys to be in deterministic order for syncing to 'state.State'
// (instertion in IAVL tree)
func (vas *TransState) Sync() {
	switch st := vas.state.(type) {
	case *TransState:
		vas.syncApp(st)
	case *TransStateWrapper:
		vas.syncState(st.state)
	default:
		panic("Unknown transState.state")
	}

}

func (vas *TransState) syncAccount(addrStr string) {
	account, deleted := unpack(vas.accounts[addrStr])
	if deleted {
		removed := vas.state.RemoveAccount(account)
		if removed != nil {
			panic(Fmt("Could not remove account to be deleted: %X. ", account.Address, removed.Error()))
		}
	} else {
		if account == nil {
			panic(Fmt("Account should not be nil for addr: %X", account.Address))
		}
		vas.state.UpdateAccount(account)
	}
}

// sync storage to blockState
func (vas *TransState) syncApp(blockState *TransState) {
	// sync account balances or remove accounts from blockState
	for addrStr := range vas.accounts {
		vas.syncAccount(addrStr)
	}

	for storageKey, value := range vas.storage {
		addrKeyBytes := []byte(storageKey)
		addr := addrKeyBytes[:32]
		if _, deleted := unpack(vas.accounts[string(addr)]); deleted {
			continue
		}
		blockState.storage[storageKey] = value
	}

	// TODO support logs, add them to the state somehow.
}

// sync storage to state.State
// CONTRACT deterministic order
func (vas *TransState) syncState(st *State) {
	// Determine order for accounts
	addrStrs := []string{}
	for addrStr := range vas.accounts {
		addrStrs = append(addrStrs, addrStr)
	}
	sort.Strings(addrStrs)

	// Update or delete accounts.
	for _, addrStr := range addrStrs {
		vas.syncAccount(addrStr)
	}

	// Determine order for storage updates
	// The address comes first so it'll be grouped.
	storageKeyStrs := []string{}
	fmt.Println("KEYS TO UPDATE:")
	for keyStr := range vas.storage {
		storageKeyStrs = append(storageKeyStrs, keyStr)
		fmt.Printf("%x, %x\n", keyStr, vas.storage[keyStr])
	}
	sort.Strings(storageKeyStrs)

	// Update storage for each account
	storage := merkle.NewIAVLTree(
		binary.BasicCodec, // TODO change
		binary.BasicCodec, // TODO change
		1024,              // TODO change.
		st.DB,
	)
	var currentAccount *vm.Account
	var deleted bool
	for _, storageKey := range storageKeyStrs {
		value := vas.storage[storageKey]
		addrKeyBytes := []byte(storageKey)
		addr := addrKeyBytes[:32]
		key := addrKeyBytes[32:]
		if currentAccount == nil || !bytes.Equal(currentAccount.Address[:], addr) {
			// if not nil, we're done processing storage for the account, so update it
			if currentAccount != nil {
				currentAccount.StorageRoot = vm.BytesToWord(storage.Hash())
				st.Sync(toStateAccount(currentAccount))
			}
			currentAccount, deleted = unpack(vas.accounts[string(addr)])
			if deleted {
				continue
			}
			var storageRoot []byte
			if currentAccount.StorageRoot.IsZero() {
				storageRoot = nil
			} else {
				storageRoot = currentAccount.StorageRoot.Bytes()
			}
			storage.Load(storageRoot)
		}
		if value.IsZero() {
			_, removed := storage.Remove(key)
			if removed != nil {
				panic(Fmt("Storage could not be removed for addr: %X @ %X. %s", addr, key, removed.Error()))
			}
		} else {
			storage.Set(key, value.Bytes())
		}

	}
	// update the last account
	if currentAccount != nil {
		hash := storage.Save()
		currentAccount.StorageRoot = vm.BytesToWord(hash)
		fmt.Printf("new storage root: %x\n", currentAccount.StorageRoot)
		st.Sync(toStateAccount(currentAccount))
	}

	if len(storageKeyStrs) > 0 {
		a := st.GetAccount([]byte(storageKeyStrs[0][:20]))
		fmt.Println("GRAB ACC: %v", a)
		fmt.Printf("adddr %x, store %x\n", storageKeyStrs[0][:20], storageKeyStrs[0][32:])
		fmt.Printf("RECO: %x\n", st.GetStorage([]byte(storageKeyStrs[0][:20]), []byte(storageKeyStrs[0][32:])))

	}
	// TODO support logs, add them to the state somehow.

}

func (vas *TransState) AddLog(log *vm.Log) {
	vas.logs = append(vas.logs, log)
}

//-----------------------------------------------------------------------------

type TransStateWrapper struct {
	state *State
}

func (app *TransStateWrapper) GetAccount(word vm.Word) (*vm.Account, error) {
	acc := app.state.GetAccount(word.Address())
	if acc == nil {
		return nil, fmt.Errorf("Account not found %x", word.Address())
	}
	return toVMAccount(acc), nil
}

func (app *TransStateWrapper) GetStorage(addr vm.Word, slot vm.Word) (vm.Word, error) {
	ret := app.state.GetStorage(addr.Address(), slot.Bytes())
	return vm.BytesToWord(ret), nil
}

func (app *TransStateWrapper) UpdateAccount(acc *vm.Account) error {
	return app.state.UpdateAccount(toStateAccount(acc))
}

func (app *TransStateWrapper) RemoveAccount(acc *vm.Account) error {
	return app.state.RemoveAccount(acc.Address.Address())
}

//-----------------------------------------------------------------------------

// Convenience function to return address of new contract
func NewContractAddress(caller []byte, nonce uint64) []byte {
	temp := make([]byte, 32+8)
	copy(temp, caller)
	vm.PutUint64(temp[32:], nonce)
	return sha3.Sha3(temp)[:20]
}
