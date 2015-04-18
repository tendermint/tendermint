package core

import (
	"fmt"
	"github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/vm"
)

func toVMAccount(acc *account.Account) *vm.Account {
	return &vm.Account{
		Address:     LeftPadWord256(acc.Address),
		Balance:     acc.Balance,
		Code:        acc.Code, // This is crazy.
		Nonce:       uint64(acc.Sequence),
		StorageRoot: RightPadWord256(acc.StorageRoot),
		Other:       acc.PubKey,
	}
}

//-----------------------------------------------------------------------------

// Run a contract's code on an isolated and unpersisted state
// Cannot be used to create new contracts
func Call(address, data []byte) (*ctypes.ResponseCall, error) {
	st := consensusState.GetState() // performs a copy
	cache := state.NewBlockCache(st)
	outAcc := cache.GetAccount(address)
	if outAcc == nil {
		return nil, fmt.Errorf("Account %x does not exist", address)
	}
	callee := toVMAccount(outAcc)
	caller := &vm.Account{Address: Zero256}
	txCache := state.NewTxCache(cache)
	params := vm.Params{
		BlockHeight: uint64(st.LastBlockHeight),
		BlockHash:   RightPadWord256(st.LastBlockHash),
		BlockTime:   st.LastBlockTime.Unix(),
		GasLimit:    10000000,
	}

	vmach := vm.NewVM(txCache, params, caller.Address, nil)
	gas := uint64(1000000000)
	ret, err := vmach.Call(caller, callee, callee.Code, data, 0, &gas)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResponseCall{Return: ret}, nil
}

// Run the given code on an isolated and unpersisted state
// Cannot be used to create new contracts
func CallCode(code, data []byte) (*ctypes.ResponseCall, error) {

	st := consensusState.GetState() // performs a copy
	cache := mempoolReactor.Mempool.GetCache()
	callee := &vm.Account{Address: Zero256}
	caller := &vm.Account{Address: Zero256}
	txCache := state.NewTxCache(cache)
	params := vm.Params{
		BlockHeight: uint64(st.LastBlockHeight),
		BlockHash:   RightPadWord256(st.LastBlockHash),
		BlockTime:   st.LastBlockTime.Unix(),
		GasLimit:    10000000,
	}

	vmach := vm.NewVM(txCache, params, caller.Address, nil)
	gas := uint64(1000000000)
	ret, err := vmach.Call(caller, callee, code, data, 0, &gas)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResponseCall{Return: ret}, nil
}

//-----------------------------------------------------------------------------

func SignTx(tx types.Tx, privAccounts []*account.PrivAccount) (*ctypes.ResponseSignTx, error) {
	// more checks?

	for i, privAccount := range privAccounts {
		if privAccount == nil || privAccount.PrivKey == nil {
			return nil, fmt.Errorf("Invalid (empty) privAccount @%v", i)
		}
	}
	switch tx.(type) {
	case *types.SendTx:
		sendTx := tx.(*types.SendTx)
		for i, input := range sendTx.Inputs {
			input.PubKey = privAccounts[i].PubKey
			input.Signature = privAccounts[i].Sign(sendTx)
		}
	case *types.CallTx:
		callTx := tx.(*types.CallTx)
		callTx.Input.PubKey = privAccounts[0].PubKey
		callTx.Input.Signature = privAccounts[0].Sign(callTx)
	case *types.BondTx:
		bondTx := tx.(*types.BondTx)
		for i, input := range bondTx.Inputs {
			input.PubKey = privAccounts[i].PubKey
			input.Signature = privAccounts[i].Sign(bondTx)
		}
	case *types.UnbondTx:
		unbondTx := tx.(*types.UnbondTx)
		unbondTx.Signature = privAccounts[0].Sign(unbondTx).(account.SignatureEd25519)
	case *types.RebondTx:
		rebondTx := tx.(*types.RebondTx)
		rebondTx.Signature = privAccounts[0].Sign(rebondTx).(account.SignatureEd25519)
	}
	return &ctypes.ResponseSignTx{tx}, nil
}
