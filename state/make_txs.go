package state

import (
	"fmt"
	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/types"
)

//----------------------------------------------------------------------------
// SendTx interface for adding inputs/outputs and adding signatures

func NewSendTx() *types.SendTx {
	return &types.SendTx{
		Inputs:  []*types.TxInput{},
		Outputs: []*types.TxOutput{},
	}
}

func SendTxAddInput(st AccountGetter, tx *types.SendTx, pubkey account.PubKey, amt uint64) error {
	addr := pubkey.Address()
	acc := st.GetAccount(addr)
	if acc == nil {
		return fmt.Errorf("Invalid address %X from pubkey %X", addr, pubkey)
	}

	tx.Inputs = append(tx.Inputs, &types.TxInput{
		Address:   addr,
		Amount:    amt,
		Sequence:  uint(acc.Sequence) + 1,
		Signature: account.SignatureEd25519{},
		PubKey:    pubkey,
	})
	return nil
}

func SendTxAddOutput(tx *types.SendTx, addr []byte, amt uint64) error {
	tx.Outputs = append(tx.Outputs, &types.TxOutput{
		Address: addr,
		Amount:  amt,
	})
	return nil
}

func SignSendTx(tx *types.SendTx, i int, privAccount *account.PrivAccount) error {
	if i >= len(tx.Inputs) {
		return fmt.Errorf("Index %v is greater than number of inputs (%v)", i, len(tx.Inputs))
	}
	tx.Inputs[i].PubKey = privAccount.PubKey
	tx.Inputs[i].Signature = privAccount.Sign(tx)
	return nil
}

//----------------------------------------------------------------------------
// CallTx interface for creating tx

func NewCallTx(st AccountGetter, from account.PubKey, to, data []byte, amt, gasLimit, fee uint64) (*types.CallTx, error) {
	addr := from.Address()
	acc := st.GetAccount(addr)
	if acc == nil {
		return nil, fmt.Errorf("Invalid address %X from pubkey %X", addr, from)
	}

	input := &types.TxInput{
		Address:   addr,
		Amount:    amt,
		Sequence:  uint(acc.Sequence) + 1,
		Signature: account.SignatureEd25519{},
		PubKey:    from,
	}

	return &types.CallTx{
		Input:    input,
		Address:  to,
		GasLimit: gasLimit,
		Fee:      fee,
		Data:     data,
	}, nil
}

func SignCallTx(tx *types.CallTx, privAccount *account.PrivAccount) {
	tx.Input.PubKey = privAccount.PubKey
	tx.Input.Signature = privAccount.Sign(tx)
}
