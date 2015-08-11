package core

import (
	"fmt"
	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/vm"
)

func toVMAccount(acc *acm.Account) *vm.Account {
	return &vm.Account{
		Address: LeftPadWord256(acc.Address),
		Balance: acc.Balance,
		Code:    acc.Code, // This is crazy.
		Nonce:   int64(acc.Sequence),
		Other:   acc.PubKey,
	}
}

//-----------------------------------------------------------------------------

// Run a contract's code on an isolated and unpersisted state
// Cannot be used to create new contracts
func Call(fromAddress, toAddress, data []byte) (*ctypes.ResultCall, error) {
	st := consensusState.GetState() // performs a copy
	cache := state.NewBlockCache(st)
	outAcc := cache.GetAccount(toAddress)
	if outAcc == nil {
		return nil, fmt.Errorf("Account %x does not exist", toAddress)
	}
	callee := toVMAccount(outAcc)
	caller := &vm.Account{Address: LeftPadWord256(fromAddress)}
	txCache := state.NewTxCache(cache)
	params := vm.Params{
		BlockHeight: int64(st.LastBlockHeight),
		BlockHash:   LeftPadWord256(st.LastBlockHash),
		BlockTime:   st.LastBlockTime.Unix(),
		GasLimit:    st.GetGasLimit(),
	}

	vmach := vm.NewVM(txCache, params, caller.Address, nil)
	gas := st.GetGasLimit()
	ret, err := vmach.Call(caller, callee, callee.Code, data, 0, &gas)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultCall{Return: ret}, nil
}

// Run the given code on an isolated and unpersisted state
// Cannot be used to create new contracts
func CallCode(fromAddress, code, data []byte) (*ctypes.ResultCall, error) {

	st := consensusState.GetState() // performs a copy
	cache := mempoolReactor.Mempool.GetCache()
	callee := &vm.Account{Address: LeftPadWord256(fromAddress)}
	caller := &vm.Account{Address: LeftPadWord256(fromAddress)}
	txCache := state.NewTxCache(cache)
	params := vm.Params{
		BlockHeight: int64(st.LastBlockHeight),
		BlockHash:   LeftPadWord256(st.LastBlockHash),
		BlockTime:   st.LastBlockTime.Unix(),
		GasLimit:    st.GetGasLimit(),
	}

	vmach := vm.NewVM(txCache, params, caller.Address, nil)
	gas := st.GetGasLimit()
	ret, err := vmach.Call(caller, callee, code, data, 0, &gas)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultCall{Return: ret}, nil
}

//-----------------------------------------------------------------------------

func SignTx(tx types.Tx, privAccounts []*acm.PrivAccount) (*ctypes.ResultSignTx, error) {
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
			input.Signature = privAccounts[i].Sign(config.GetString("chain_id"), sendTx)
		}
	case *types.CallTx:
		callTx := tx.(*types.CallTx)
		callTx.Input.PubKey = privAccounts[0].PubKey
		callTx.Input.Signature = privAccounts[0].Sign(config.GetString("chain_id"), callTx)
	case *types.BondTx:
		bondTx := tx.(*types.BondTx)
		// the first privaccount corresponds to the BondTx pub key.
		// the rest to the inputs
		bondTx.Signature = privAccounts[0].Sign(config.GetString("chain_id"), bondTx).(acm.SignatureEd25519)
		for i, input := range bondTx.Inputs {
			input.PubKey = privAccounts[i+1].PubKey
			input.Signature = privAccounts[i+1].Sign(config.GetString("chain_id"), bondTx)
		}
	case *types.UnbondTx:
		unbondTx := tx.(*types.UnbondTx)
		unbondTx.Signature = privAccounts[0].Sign(config.GetString("chain_id"), unbondTx).(acm.SignatureEd25519)
	case *types.RebondTx:
		rebondTx := tx.(*types.RebondTx)
		rebondTx.Signature = privAccounts[0].Sign(config.GetString("chain_id"), rebondTx).(acm.SignatureEd25519)
	}
	return &ctypes.ResultSignTx{tx}, nil
}
