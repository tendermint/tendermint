package blocks

import (
	. "github.com/tendermint/tendermint/binary"
	"io"
)

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

func ReadSignatures(r io.Reader, n *int64, err *error) (sigs []Signature) {
	length := ReadUInt32(r, n, err)
	for i := uint32(0); i < length; i++ {
		sigs = append(sigs, ReadSignature(r, n, err))
	}
	return
}

func WriteSignatures(w io.Writer, sigs []Signature, n *int64, err *error) {
	WriteUInt32(w, uint32(len(sigs)), n, err)
	for _, sig := range sigs {
		WriteBinary(w, sig, n, err)
		if *err != nil {
			return
		}
	}
}
