package blocks

import (
	. "github.com/tendermint/tendermint/binary"
	"io"
)

// NOTE: consensus/Validator embeds this, so..
type Account struct {
	Id     uint64 // Numeric id of account, incrementing.
	PubKey []byte
}

func (self *Account) Verify(msg []byte, sig []byte) bool {
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

//-----------------------------------------------------------------------------

/*
Signature message wire format:

    |A...|SSS...|

    A  account number, varint encoded (1+ bytes)
    S  signature of all prior bytes (32 bytes)

It usually follows the message to be signed.

*/

type Signature struct {
	SignerId uint64
	Bytes    []byte
}

func ReadSignature(r io.Reader, n *int64, err *error) Signature {
	return Signature{
		SignerId: ReadUInt64(r, n, err),
		Bytes:    ReadByteSlice(r, n, err),
	}
}

func (sig Signature) IsZero() bool {
	return len(sig.Bytes) == 0
}

func (sig Signature) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt64(w, sig.SignerId, &n, &err)
	WriteByteSlice(w, sig.Bytes, &n, &err)
	return
}
