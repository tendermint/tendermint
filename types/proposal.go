package types

import (
	"errors"
	"fmt"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	tmtime "github.com/tendermint/tendermint/types/time"
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
	Type             SignedMsgType
	Height           int64         `json:"height"`
	Round            int           `json:"round"`
	POLRound         int           `json:"pol_round"` // -1 if null.
	Timestamp        time.Time     `json:"timestamp"`
	BlockPartsHeader PartSetHeader `json:"block_parts_header"`
	POLBlockID       BlockID       `json:"pol_block_id"` // zero if null.
	Signature        []byte        `json:"signature"`
}

// NewProposal returns a new Proposal.
// If there is no POLRound, polRound should be -1.
func NewProposal(height int64, round int, blockPartsHeader PartSetHeader, polRound int, polBlockID BlockID) *Proposal {
	return &Proposal{
		Type:             ProposalType,
		Height:           height,
		Round:            round,
		POLRound:         polRound,
		Timestamp:        tmtime.Now(),
		BlockPartsHeader: blockPartsHeader,
		POLBlockID:       polBlockID,
	}
}

// ValidateBasic performs basic validation.
func (p *Proposal) ValidateBasic() error {
	if p.Height < 0 {
		return errors.New("Negative Height")
	}
	if p.Round < 0 {
		return errors.New("Negative Round")
	}
	if err := p.BlockPartsHeader.ValidateBasic(); err != nil {
		return fmt.Errorf("Wrong BlockPartsHeader: %v", err)
	}
	if p.POLRound < -1 {
		return errors.New("Negative POLRound (exception: -1)")
	}
	if err := p.POLBlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("Wrong POLBlockID: %v", err)
	}
	if len(p.Signature) == 0 {
		return errors.New("Signature is missing")
	}
	if len(p.Signature) > MaxSignatureSize {
		return fmt.Errorf("Signature is too big (max: %d)", MaxSignatureSize)
	}
	return nil
}

// String returns a string representation of the Proposal.
func (p *Proposal) String() string {
	return fmt.Sprintf("Proposal{%v/%v %v (%v,%v) %X @ %s}",
		p.Height,
		p.Round,
		p.BlockPartsHeader,
		p.POLRound,
		p.POLBlockID,
		cmn.Fingerprint(p.Signature),
		CanonicalTime(p.Timestamp))
}

// SignBytes returns the Proposal bytes for signing
func (p *Proposal) SignBytes(chainID string) []byte {
	bz, err := cdc.MarshalBinaryLengthPrefixed(CanonicalizeProposal(chainID, p))
	if err != nil {
		panic(err)
	}
	return bz
}
