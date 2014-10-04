package state

import (
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/crypto"
	"io"
)

type Account struct {
	Id     uint64 // Numeric id of account, incrementing.
	PubKey []byte
}

func ReadAccount(r io.Reader, n *int64, err *error) *Account {
	return &Account{
		Id:     ReadUInt64(r, n, err),
		PubKey: ReadByteSlice(r, n, err),
	}
}

func (account *Account) Verify(msg []byte, sig Signature) bool {
	if sig.SignerId != account.Id {
		panic("Account.Id doesn't match sig.SignerId")
	}
	v1 := &crypto.Verify{
		Message:   msg,
		PubKey:    account.PubKey,
		Signature: sig.Bytes,
	}
	ok := crypto.VerifyBatch([]*crypto.Verify{v1})
	return ok
}

//-----------------------------------------------------------------------------

type AccountBalance struct {
	Account
	Balance uint64
}

//-----------------------------------------------------------------------------

type PrivAccount struct {
	Account
	PrivKey []byte
}

// Generates a new account with private key.
// The Account.Id is empty since it isn't in the blockchain.
func GenPrivAccount() *PrivAccount {
	privKey := RandBytes(32)
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
	return Signature{
		SignerId: pa.Id,
		Bytes:    signature,
	}
}
