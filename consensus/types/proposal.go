package consensus

import (
	"errors"
	"fmt"
	"io"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/config"
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
	POLParts   types.PartSetHeader      `json:"pol_parts"`
	Signature  account.SignatureEd25519 `json:"signature"`
}

func NewProposal(height uint, round uint, blockParts, polParts types.PartSetHeader) *Proposal {
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
	// We hex encode the network name so we don't deal with escaping issues.
	binary.WriteTo([]byte(Fmt(`{"network":"%X"`, config.App().GetString("Network"))), w, n, err)
	binary.WriteTo([]byte(`,"proposal":{"block_parts":`), w, n, err)
	p.BlockParts.WriteSignBytes(w, n, err)
	binary.WriteTo([]byte(Fmt(`,"height":%v,"pol_parts":`, p.Height)), w, n, err)
	p.POLParts.WriteSignBytes(w, n, err)
	binary.WriteTo([]byte(Fmt(`,"round":%v}}`, p.Round)), w, n, err)
}
