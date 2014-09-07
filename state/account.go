package state

import (
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	"io"
)

// NOTE: consensus/Validator embeds this, so..
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

func (self *Account) Verify(msg []byte, sig Signature) bool {
	if sig.SignerId != self.Id {
		return false
	}
	return false
}

//-----------------------------------------------------------------------------

type PrivAccount struct {
	Account
	PrivKey []byte
}

func (self *PrivAccount) Sign(msg []byte) Signature {
	return Signature{}
}
