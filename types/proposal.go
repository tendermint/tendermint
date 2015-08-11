package types

import (
	"errors"
	"fmt"
	"io"

	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/wire"
)

var (
	ErrInvalidBlockPartSignature = errors.New("Error invalid block part signature")
	ErrInvalidBlockPartHash      = errors.New("Error invalid block part hash")
)

type Proposal struct {
	Height           int                  `json:"height"`
	Round            int                  `json:"round"`
	BlockPartsHeader PartSetHeader        `json:"block_parts_header"`
	POLRound         int                  `json:"pol_round"` // -1 if null.
	Signature        acm.SignatureEd25519 `json:"signature"`
}

func NewProposal(height int, round int, blockPartsHeader PartSetHeader, polRound int) *Proposal {
	return &Proposal{
		Height:           height,
		Round:            round,
		BlockPartsHeader: blockPartsHeader,
		POLRound:         polRound,
	}
}

func (p *Proposal) String() string {
	return fmt.Sprintf("Proposal{%v/%v %v %v %v}", p.Height, p.Round,
		p.BlockPartsHeader, p.POLRound, p.Signature)
}

func (p *Proposal) WriteSignBytes(chainID string, w io.Writer, n *int64, err *error) {
	wire.WriteTo([]byte(Fmt(`{"chain_id":"%s"`, chainID)), w, n, err)
	wire.WriteTo([]byte(`,"proposal":{"block_parts_header":`), w, n, err)
	p.BlockPartsHeader.WriteSignBytes(w, n, err)
	wire.WriteTo([]byte(Fmt(`,"height":%v,"pol_round":%v`, p.Height, p.POLRound)), w, n, err)
	wire.WriteTo([]byte(Fmt(`,"round":%v}}`, p.Round)), w, n, err)
}
