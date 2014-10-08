package state

import (
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/crypto"
	"io"
)

const (
	AccountBalanceStatusNominal = byte(0x00)
	AccountBalanceStatusBonded  = byte(0x01)
)

type Account struct {
	Id     uint64 // Numeric id of account, incrementing.
	PubKey []byte
}

func ReadAccount(r io.Reader, n *int64, err *error) Account {
	return Account{
		Id:     ReadUInt64(r, n, err),
		PubKey: ReadByteSlice(r, n, err),
	}
}

func (account Account) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt64(w, account.Id, &n, &err)
	WriteByteSlice(w, account.PubKey, &n, &err)
	return
}

func (account Account) Verify(msg []byte, sig Signature) bool {
	if sig.SignerId != account.Id {
		panic("account.id doesn't match sig.signerid")
	}
	v1 := &crypto.Verify{
		Message:   msg,
		PubKey:    account.PubKey,
		Signature: sig.Bytes,
	}
	ok := crypto.VerifyBatch([]*crypto.Verify{v1})
	return ok
}

func (account Account) VerifySignable(o Signable) bool {
	msg := o.GenDocument()
	sig := o.GetSignature()
	return account.Verify(msg, sig)
}

//-----------------------------------------------------------------------------

type AccountBalance struct {
	Account
	Balance uint64
	Status  byte
}

func ReadAccountBalance(r io.Reader, n *int64, err *error) *AccountBalance {
	return &AccountBalance{
		Account: ReadAccount(r, n, err),
		Balance: ReadUInt64(r, n, err),
		Status:  ReadByte(r, n, err),
	}
}

func (accBal AccountBalance) WriteTo(w io.Writer) (n int64, err error) {
	WriteBinary(w, accBal.Account, &n, &err)
	WriteUInt64(w, accBal.Balance, &n, &err)
	WriteByte(w, accBal.Status, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type PrivAccount struct {
	Account
	PrivKey []byte
}

// Generates a new account with private key.
// The Account.Id is empty since it isn't in the blockchain.
func GenPrivAccount() *PrivAccount {
	privKey := CRandBytes(32)
	pubKey := crypto.MakePubKey(privKey)
	return &PrivAccount{
		Account: Account{
			Id:     uint64(0),
			PubKey: pubKey,
		},
		PrivKey: privKey,
	}
}

func (pa *PrivAccount) Sign(msg []byte) Signature {
	signature := crypto.SignMessage(msg, pa.PrivKey, pa.PubKey)
	sig := Signature{
		SignerId: pa.Id,
		Bytes:    signature,
	}
	return sig
}

func (pa *PrivAccount) SignSignable(o Signable) {
	msg := o.GenDocument()
	sig := pa.Sign(msg)
	o.SetSignature(sig)
}
