package consensus

import (
	"errors"
	"fmt"
	"io"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
)

var (
	ErrInvalidBlockPartSignature = errors.New("Error invalid block part signature")
	ErrInvalidBlockPartHash      = errors.New("Error invalid block part hash")
)

type Proposal struct {
	Height     uint32
	Round      uint16
	BlockParts PartSetHeader
	POLParts   PartSetHeader
	Signature  Signature
}

func NewProposal(height uint32, round uint16, blockParts, polParts PartSetHeader) *Proposal {

	return &Proposal{
		Height:     height,
		Round:      round,
		BlockParts: blockParts,
		POLParts:   polParts,
	}
}

func ReadProposal(r io.Reader, n *int64, err *error) *Proposal {
	return &Proposal{
		Height:     ReadUInt32(r, n, err),
		Round:      ReadUInt16(r, n, err),
		BlockParts: ReadPartSetHeader(r, n, err),
		POLParts:   ReadPartSetHeader(r, n, err),
		Signature:  ReadSignature(r, n, err),
	}
}

func (p *Proposal) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt32(w, p.Height, &n, &err)
	WriteUInt16(w, p.Round, &n, &err)
	WriteBinary(w, p.BlockParts, &n, &err)
	WriteBinary(w, p.POLParts, &n, &err)
	WriteBinary(w, p.Signature, &n, &err)
	return
}

func (p *Proposal) GetSignature() Signature {
	return p.Signature
}

func (p *Proposal) SetSignature(sig Signature) {
	p.Signature = sig
}

func (p *Proposal) String() string {
	return fmt.Sprintf("Proposal{%v/%v %v %v %v}", p.Height, p.Round,
		p.BlockParts, p.POLParts, p.Signature)
}
