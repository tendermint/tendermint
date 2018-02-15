package types

import (
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/wire"
)

var (
	ErrInvalidBlockPartSignature = errors.New("Error invalid block part signature")
	ErrInvalidBlockPartHash      = errors.New("Error invalid block part hash")
)

// Proposal defines a block proposal for the consensus.
// It refers to the block only by its PartSetHeader.
// It must be signed by the correct proposer for the given Height/Round
// to be considered valid. It may depend on votes from a previous round,
// a so-called Proof-of-Lock (POL) round, as noted in the POLRound and POLBlockID.
type Proposal struct {
	Height           int64            `json:"height"`
	Round            int              `json:"round"`
	Timestamp        time.Time        `json:"timestamp"`
	BlockPartsHeader PartSetHeader    `json:"block_parts_header"`
	POLRound         int              `json:"pol_round"`    // -1 if null.
	POLBlockID       BlockID          `json:"pol_block_id"` // zero if null.
	Signature        crypto.Signature `json:"signature"`
}

// NewProposal returns a new Proposal.
// If there is no POLRound, polRound should be -1.
func NewProposal(height int64, round int, blockPartsHeader PartSetHeader, polRound int, polBlockID BlockID) *Proposal {
	return &Proposal{
		Height:           height,
		Round:            round,
		Timestamp:        time.Now().UTC(),
		BlockPartsHeader: blockPartsHeader,
		POLRound:         polRound,
		POLBlockID:       polBlockID,
	}
}

// String returns a string representation of the Proposal.
func (p *Proposal) String() string {
	return fmt.Sprintf("Proposal{%v/%v %v (%v,%v) %v @ %s}",
		p.Height, p.Round, p.BlockPartsHeader, p.POLRound,
		p.POLBlockID, p.Signature, CanonicalTime(p.Timestamp))
}

// SignBytes returns the Proposal bytes for signing
func (p *Proposal) SignBytes(chainID string) []byte {
	bz, err := wire.MarshalJSON(CanonicalJSONOnceProposal{
		ChainID:  chainID,
		Proposal: CanonicalProposal(p),
	})
	if err != nil {
		panic(err)
	}
	return bz
}
