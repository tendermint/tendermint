package consensus

import (
	"errors"
	"fmt"
	"io"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/block"
)

var (
	ErrInvalidBlockPartSignature = errors.New("Error invalid block part signature")
	ErrInvalidBlockPartHash      = errors.New("Error invalid block part hash")
)

type Proposal struct {
	Height     uint
	Round      uint
	BlockParts PartSetHeader
	POLParts   PartSetHeader
	Signature  SignatureEd25519
}

func NewProposal(height uint, round uint, blockParts, polParts PartSetHeader) *Proposal {
	return &Proposal{
		Height:     height,
		Round:      round,
		BlockParts: blockParts,
		POLParts:   polParts,
	}
}

func (p *Proposal) String() string {
	return fmt.Sprintf("Proposal{%v/%v %v %v %v}", p.Height, p.Round,
		p.BlockParts, p.POLParts, p.Signature)
}

func (p *Proposal) WriteSignBytes(w io.Writer, n *int64, err *error) {
	WriteUVarInt(p.Height, w, n, err)
	WriteUVarInt(p.Round, w, n, err)
	WriteBinary(p.BlockParts, w, n, err)
	WriteBinary(p.POLParts, w, n, err)
}
