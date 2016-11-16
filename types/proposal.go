package types

import (
	"errors"
	"fmt"
	"io"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"
)

var (
	ErrInvalidBlockPartSignature = errors.New("Error invalid block part signature")
	ErrInvalidBlockPartHash      = errors.New("Error invalid block part hash")
)

type Proposal struct {
	Height           int                     `json:"height"`
	Round            int                     `json:"round"`
	BlockPartsHeader PartSetHeader           `json:"block_parts_header"`
	POLRound         int                     `json:"pol_round"`    // -1 if null.
	POLBlockID       BlockID                 `json:"pol_block_id"` // zero if null.
	Signature        crypto.SignatureEd25519 `json:"signature"`
}

// polRound: -1 if no polRound.
func NewProposal(height int, round int, blockPartsHeader PartSetHeader, polRound int, polBlockID BlockID) *Proposal {
	return &Proposal{
		Height:           height,
		Round:            round,
		BlockPartsHeader: blockPartsHeader,
		POLRound:         polRound,
		POLBlockID:       polBlockID,
	}
}

func (p *Proposal) String() string {
	return fmt.Sprintf("Proposal{%v/%v %v (%v,%v) %v}", p.Height, p.Round,
		p.BlockPartsHeader, p.POLRound, p.POLBlockID, p.Signature)
}

func (p *Proposal) WriteSignBytes(chainID string, w io.Writer, n *int, err *error) {
	wire.WriteTo([]byte(Fmt(`{"chain_id":"%s"`, chainID)), w, n, err)
	wire.WriteTo([]byte(`,"proposal":{"block_parts_header":`), w, n, err)
	p.BlockPartsHeader.WriteSignBytes(w, n, err)
	wire.WriteTo([]byte(Fmt(`,"height":%v,"pol_block_id":`, p.Height)), w, n, err)
	p.POLBlockID.WriteSignBytes(w, n, err)
	wire.WriteTo([]byte(Fmt(`,"pol_round":%v`, p.POLRound)), w, n, err)
	wire.WriteTo([]byte(Fmt(`,"round":%v}}`, p.Round)), w, n, err)
}
