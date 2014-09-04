package blocks

import (
	. "github.com/tendermint/tendermint/binary"
	"io"
)

/*
Signature message wire format:

    |a...|sss...|

    a  Account number, varint encoded (1+ bytes)
    s  Signature of all prior bytes (32 bytes)

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
