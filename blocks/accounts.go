package blocks

import (
	. "github.com/tendermint/tendermint/binary"
	"io"
)

// NOTE: consensus/Validator embeds this, so..
type Account struct {
	Id     UInt64 // Numeric id of account, incrementing.
	PubKey ByteSlice
}

func (self *Account) Verify(msg ByteSlice, sig ByteSlice) bool {
	return false
}

//-----------------------------------------------------------------------------

type PrivAccount struct {
	Account
	PrivKey ByteSlice
}

func (self *PrivAccount) Sign(msg ByteSlice) Signature {
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
	SignerId UInt64
	Bytes    ByteSlice
}

func ReadSignature(r io.Reader) Signature {
	return Signature{
		SignerId: ReadUInt64(r),
		Bytes:    ReadByteSlice(r),
	}
}

func (sig Signature) IsZero() bool {
	return len(sig.Bytes) == 0
}

func (sig Signature) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(sig.SignerId, w, n, err)
	n, err = WriteTo(sig.Bytes, w, n, err)
	return
}
