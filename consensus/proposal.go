package consensus

import (
	"errors"
	"io"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
)

var (
	ErrInvalidBlockPartSignature = errors.New("Error invalid block part signature")
	ErrInvalidBlockPartHash      = errors.New("Error invalid block part hash")
)

type Proposal struct {
	Height          uint32
	Round           uint16
	BlockPartsTotal uint16
	BlockPartsHash  []byte
	POLPartsTotal   uint16
	POLPartsHash    []byte
	Signature
}

func NewProposal(height uint32, round uint16, blockPartsTotal uint16, blockPartsHash []byte,
	polPartsTotal uint16, polPartsHash []byte) *Proposal {
	return &Proposal{
		Height:          height,
		Round:           round,
		BlockPartsTotal: blockPartsTotal,
		BlockPartsHash:  blockPartsHash,
		POLPartsTotal:   polPartsTotal,
		POLPartsHash:    polPartsHash,
	}
}

func ReadProposal(r io.Reader, n *int64, err *error) *Proposal {
	return &Proposal{
		Height:          ReadUInt32(r, n, err),
		Round:           ReadUInt16(r, n, err),
		BlockPartsTotal: ReadUInt16(r, n, err),
		BlockPartsHash:  ReadByteSlice(r, n, err),
		POLPartsTotal:   ReadUInt16(r, n, err),
		POLPartsHash:    ReadByteSlice(r, n, err),
		Signature:       ReadSignature(r, n, err),
	}
}

func (p *Proposal) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt32(w, p.Height, &n, &err)
	WriteUInt16(w, p.Round, &n, &err)
	WriteUInt16(w, p.BlockPartsTotal, &n, &err)
	WriteByteSlice(w, p.BlockPartsHash, &n, &err)
	WriteUInt16(w, p.POLPartsTotal, &n, &err)
	WriteByteSlice(w, p.POLPartsHash, &n, &err)
	WriteBinary(w, p.Signature, &n, &err)
	return
}

func (p *Proposal) GenDocument() []byte {
	oldSig := p.Signature
	p.Signature = Signature{}
	doc := BinaryBytes(p)
	p.Signature = oldSig
	return doc
}

func (p *Proposal) GetSignature() Signature {
	return p.Signature
}

func (p *Proposal) SetSignature(sig Signature) {
	p.Signature = sig
}
