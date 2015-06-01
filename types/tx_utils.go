package types

import (
	"fmt"
	"github.com/tendermint/tendermint/account"
)

type AccountGetter interface {
	GetAccount(addr []byte) *account.Account
}

//----------------------------------------------------------------------------
// SendTx interface for adding inputs/outputs and adding signatures

func NewSendTx() *SendTx {
	return &SendTx{
		Inputs:  []*TxInput{},
		Outputs: []*TxOutput{},
	}
}

func (tx *SendTx) AddInput(st AccountGetter, pubkey account.PubKey, amt uint64) error {
	addr := pubkey.Address()
	acc := st.GetAccount(addr)
	if acc == nil {
		return fmt.Errorf("Invalid address %X from pubkey %X", addr, pubkey)
	}
	return tx.AddInputWithNonce(pubkey, amt, acc.Sequence+1)
}

func (tx *SendTx) AddInputWithNonce(pubkey account.PubKey, amt uint64, nonce uint) error {
	addr := pubkey.Address()
	tx.Inputs = append(tx.Inputs, &TxInput{
		Address:   addr,
		Amount:    amt,
		Sequence:  nonce,
		Signature: account.SignatureEd25519{},
		PubKey:    pubkey,
	})
	return nil
}

func (tx *SendTx) AddOutput(addr []byte, amt uint64) error {
	tx.Outputs = append(tx.Outputs, &TxOutput{
		Address: addr,
		Amount:  amt,
	})
	return nil
}

func (tx *SendTx) SignInput(chainID string, i int, privAccount *account.PrivAccount) error {
	if i >= len(tx.Inputs) {
		return fmt.Errorf("Index %v is greater than number of inputs (%v)", i, len(tx.Inputs))
	}
	tx.Inputs[i].PubKey = privAccount.PubKey
	tx.Inputs[i].Signature = privAccount.Sign(chainID, tx)
	return nil
}

//----------------------------------------------------------------------------
// CallTx interface for creating tx

func NewCallTx(st AccountGetter, from account.PubKey, to, data []byte, amt, gasLimit, fee uint64) (*CallTx, error) {
	addr := from.Address()
	acc := st.GetAccount(addr)
	if acc == nil {
		return nil, fmt.Errorf("Invalid address %X from pubkey %X", addr, from)
	}

	nonce := acc.Sequence + 1
	return NewCallTxWithNonce(from, to, data, amt, gasLimit, fee, nonce), nil
}

func NewCallTxWithNonce(from account.PubKey, to, data []byte, amt, gasLimit, fee uint64, nonce uint) *CallTx {
	addr := from.Address()
	input := &TxInput{
		Address:   addr,
		Amount:    amt,
		Sequence:  nonce,
		Signature: account.SignatureEd25519{},
		PubKey:    from,
	}

	return &CallTx{
		Input:    input,
		Address:  to,
		GasLimit: gasLimit,
		Fee:      fee,
		Data:     data,
	}
}

func (tx *CallTx) Sign(chainID string, privAccount *account.PrivAccount) {
	tx.Input.PubKey = privAccount.PubKey
	tx.Input.Signature = privAccount.Sign(chainID, tx)
}

//----------------------------------------------------------------------------
// NameTx interface for creating tx

func NewNameTx(st AccountGetter, from account.PubKey, name, data string, amt, fee uint64) (*NameTx, error) {
	addr := from.Address()
	acc := st.GetAccount(addr)
	if acc == nil {
		return nil, fmt.Errorf("Invalid address %X from pubkey %X", addr, from)
	}

	nonce := acc.Sequence + 1
	return NewNameTxWithNonce(from, name, data, amt, fee, nonce), nil
}

func NewNameTxWithNonce(from account.PubKey, name, data string, amt, fee uint64, nonce uint) *NameTx {
	addr := from.Address()
	input := &TxInput{
		Address:   addr,
		Amount:    amt,
		Sequence:  nonce,
		Signature: account.SignatureEd25519{},
		PubKey:    from,
	}

	return &NameTx{
		Input: input,
		Name:  name,
		Data:  data,
		Fee:   fee,
	}
}

func (tx *NameTx) Sign(chainID string, privAccount *account.PrivAccount) {
	tx.Input.PubKey = privAccount.PubKey
	tx.Input.Signature = privAccount.Sign(chainID, tx)
}

//----------------------------------------------------------------------------
// BondTx interface for adding inputs/outputs and adding signatures

func NewBondTx(pubkey account.PubKey) (*BondTx, error) {
	pubkeyEd, ok := pubkey.(account.PubKeyEd25519)
	if !ok {
		return nil, fmt.Errorf("Pubkey must be ed25519")
	}
	return &BondTx{
		PubKey:   pubkeyEd,
		Inputs:   []*TxInput{},
		UnbondTo: []*TxOutput{},
	}, nil
}

func (tx *BondTx) AddInput(st AccountGetter, pubkey account.PubKey, amt uint64) error {
	addr := pubkey.Address()
	acc := st.GetAccount(addr)
	if acc == nil {
		return fmt.Errorf("Invalid address %X from pubkey %X", addr, pubkey)
	}
	return tx.AddInputWithNonce(pubkey, amt, acc.Sequence+1)
}

func (tx *BondTx) AddInputWithNonce(pubkey account.PubKey, amt uint64, nonce uint) error {
	addr := pubkey.Address()
	tx.Inputs = append(tx.Inputs, &TxInput{
		Address:   addr,
		Amount:    amt,
		Sequence:  nonce,
		Signature: account.SignatureEd25519{},
		PubKey:    pubkey,
	})
	return nil
}

func (tx *BondTx) AddOutput(addr []byte, amt uint64) error {
	tx.UnbondTo = append(tx.UnbondTo, &TxOutput{
		Address: addr,
		Amount:  amt,
	})
	return nil
}

func (tx *BondTx) SignBond(chainID string, privAccount *account.PrivAccount) error {
	sig := privAccount.Sign(chainID, tx)
	sigEd, ok := sig.(account.SignatureEd25519)
	if !ok {
		return fmt.Errorf("Bond signer must be ED25519")
	}
	tx.Signature = sigEd
	return nil
}

func (tx *BondTx) SignInput(chainID string, i int, privAccount *account.PrivAccount) error {
	if i >= len(tx.Inputs) {
		return fmt.Errorf("Index %v is greater than number of inputs (%v)", i, len(tx.Inputs))
	}
	tx.Inputs[i].PubKey = privAccount.PubKey
	tx.Inputs[i].Signature = privAccount.Sign(chainID, tx)
	return nil
}
