package blocks

import (
	. "github.com/tendermint/tendermint/binary"
	"io"
)

/*

Signature message wire format:

    |A...|SSS...|

    A  account number, varint encoded (1+ bytes)
    S  signature of all prior bytes (32 bytes)

It usually follows the message to be signed.

*/

type Signature struct {
	Signer   AccountId
	SigBytes ByteSlice
}

func ReadSignature(r io.Reader) Signature {
	return Signature{
		Signer:   ReadAccountId(r),
		SigBytes: ReadByteSlice(r),
	}
}

func (self Signature) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(self.Signer, w, n, err)
	n, err = WriteOnto(self.SigBytes, w, n, err)
	return
}

func (self *Signature) Verify(msg ByteSlice) bool {
	return false
}
