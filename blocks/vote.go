package blocks

import (
	. "github.com/tendermint/tendermint/binary"
	"io"
)

/*
The full vote structure is only needed when presented as evidence.
Typically only the signature is passed around, as the hash & height are implied.
*/

type Vote struct {
	Height    UInt64
	BlockHash ByteSlice
	Signature
}

func ReadVote(r io.Reader) Vote {
	return Vote{
		Height:    ReadUInt64(r),
		BlockHash: ReadByteSlice(r),
		Signature: ReadSignature(r),
	}
}

func (self Vote) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(self.Height, w, n, err)
	n, err = WriteOnto(self.BlockHash, w, n, err)
	n, err = WriteOnto(self.Signature, w, n, err)
	return
}
