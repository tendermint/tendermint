package blocks

import (
	"fmt"
	"io"

	. "github.com/tendermint/tendermint/binary"
)

type Signable interface {
	Binary
	GetSignature() Signature
	SetSignature(Signature)
}

//-----------------------------------------------------------------------------

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

func (sig Signature) String() string {
	return fmt.Sprintf("Signature{%v:%X}", sig.SignerId, sig.Bytes)
}

//-------------------------------------

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

//-----------------------------------------------------------------------------

type RoundSignature struct {
	Round uint16
	Signature
}

func ReadRoundSignature(r io.Reader, n *int64, err *error) RoundSignature {
	return RoundSignature{
		ReadUInt16(r, n, err),
		ReadSignature(r, n, err),
	}
}

func (rsig RoundSignature) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt16(w, rsig.Round, &n, &err)
	WriteBinary(w, rsig.Signature, &n, &err)
	return
}

func (rsig RoundSignature) IsZero() bool {
	return rsig.Round == 0 && rsig.SignerId == 0 && len(rsig.Bytes) == 0
}

//-------------------------------------

func ReadRoundSignatures(r io.Reader, n *int64, err *error) (rsigs []RoundSignature) {
	length := ReadUInt32(r, n, err)
	for i := uint32(0); i < length; i++ {
		rsigs = append(rsigs, ReadRoundSignature(r, n, err))
	}
	return
}

func WriteRoundSignatures(w io.Writer, rsigs []RoundSignature, n *int64, err *error) {
	WriteUInt32(w, uint32(len(rsigs)), n, err)
	for _, rsig := range rsigs {
		WriteBinary(w, rsig, n, err)
		if *err != nil {
			return
		}
	}
}
