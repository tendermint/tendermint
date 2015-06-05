package consensus

import (
	"errors"
	"fmt"
	"io"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/types"
)

var (
	ErrInvalidBlockPartSignature = errors.New("Error invalid block part signature")
	ErrInvalidBlockPartHash      = errors.New("Error invalid block part hash")
)

type Proposal struct {
	Height     uint                     `json:"height"`
	Round      uint                     `json:"round"`
	BlockParts types.PartSetHeader      `json:"block_parts"`
	POLRound   int                      `json:"pol_round"` // -1 if null.
	Signature  account.SignatureEd25519 `json:"signature"`
}

func NewProposal(height uint, round uint, blockParts types.PartSetHeader, polRound int) *Proposal {
	return &Proposal{
		Height:     height,
		Round:      round,
		BlockParts: blockParts,
		POLRound:   polRound,
	}
}

func (p *Proposal) String() string {
	return fmt.Sprintf("Proposal{%v/%v %v %v %v}", p.Height, p.Round,
		p.BlockParts, p.POLRound, p.Signature)
}

func (p *Proposal) WriteSignBytes(chainID string, w io.Writer, n *int64, err *error) {
	binary.WriteTo([]byte(Fmt(`{"chain_id":"%s"`, chainID)), w, n, err)
	binary.WriteTo([]byte(`,"proposal":{"block_parts":`), w, n, err)
	p.BlockParts.WriteSignBytes(w, n, err)
	binary.WriteTo([]byte(Fmt(`,"height":%v,"pol_round":%v`, p.Height, p.POLRound)), w, n, err)
	binary.WriteTo([]byte(Fmt(`,"round":%v}}`, p.Round)), w, n, err)
}
