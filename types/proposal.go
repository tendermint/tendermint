package types

import (
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/libs/bytes"
	tmproto "github.com/tendermint/tendermint/proto/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

var (
	ErrInvalidBlockPartSignature = errors.New("error invalid block part signature")
	ErrInvalidBlockPartHash      = errors.New("error invalid block part hash")
)

// Proposal defines a block proposal for the consensus.
// It refers to the block by BlockID field.
// It must be signed by the correct proposer for the given Height/Round
// to be considered valid. It may depend on votes from a previous round,
// a so-called Proof-of-Lock (POL) round, as noted in the POLRound.
// If POLRound >= 0, then BlockID corresponds to the block that is locked in POLRound.
type Proposal struct {
	Type      SignedMsgType
	Height    int64     `json:"height"`
	Round     int       `json:"round"`
	POLRound  int       `json:"pol_round"` // -1 if null.
	BlockID   BlockID   `json:"block_id"`
	Timestamp time.Time `json:"timestamp"`
	Signature []byte    `json:"signature"`
}

// NewProposal returns a new Proposal.
// If there is no POLRound, polRound should be -1.
func NewProposal(height int64, round int, polRound int, blockID BlockID) *Proposal {
	return &Proposal{
		Type:      ProposalType,
		Height:    height,
		Round:     round,
		BlockID:   blockID,
		POLRound:  polRound,
		Timestamp: tmtime.Now(),
	}
}

// ValidateBasic performs basic validation.
func (p *Proposal) ValidateBasic() error {
	if p.Type != ProposalType {
		return errors.New("invalid Type")
	}
	if p.Height < 0 {
		return errors.New("negative Height")
	}
	if p.Round < 0 {
		return errors.New("negative Round")
	}
	if p.POLRound < -1 {
		return errors.New("negative POLRound (exception: -1)")
	}
	if err := p.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockID: %v", err)
	}
	// ValidateBasic above would pass even if the BlockID was empty:
	if !p.BlockID.IsComplete() {
		return fmt.Errorf("expected a complete, non-empty BlockID, got: %v", p.BlockID)
	}

	// NOTE: Timestamp validation is subtle and handled elsewhere.

	if len(p.Signature) == 0 {
		return errors.New("signature is missing")
	}
	if len(p.Signature) > MaxSignatureSize {
		return fmt.Errorf("signature is too big (max: %d)", MaxSignatureSize)
	}
	return nil
}

// String returns a string representation of the Proposal.
func (p *Proposal) String() string {
	return fmt.Sprintf("Proposal{%v/%v (%v, %v) %X @ %s}",
		p.Height,
		p.Round,
		p.BlockID,
		p.POLRound,
		bytes.Fingerprint(p.Signature),
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

// ToProto converts Proposal to protobuf
func (p *Proposal) ToProto() *tmproto.Proposal {
	if p == nil {
		return nil
	}
	pb := new(tmproto.Proposal)

	pb.BlockID = p.BlockID.ToProto()
	pb.Type = tmproto.SignedMsgType(p.Type)
	pb.Height = p.Height
	pb.Round = int32(p.Round)
	pb.PolRound = int32(p.POLRound)
	pb.Timestamp = p.Timestamp
	pb.Signature = p.Signature

	return pb
}

// FromProto sets a protobuf Proposal to the given pointer.
// It returns an error if the proposal is invalid.
func ProposalFromProto(pp *tmproto.Proposal) (*Proposal, error) {
	if pp == nil {
		return nil, errors.New("nil proposal")
	}

	p := new(Proposal)

	blockID, err := BlockIDFromProto(&pp.BlockID)
	if err != nil {
		return nil, err
	}

	p.BlockID = *blockID
	p.Type = SignedMsgType(pp.Type)
	p.Height = pp.Height
	p.Round = int(pp.Round)
	p.POLRound = int(pp.PolRound)
	p.Timestamp = pp.Timestamp
	p.Signature = pp.Signature

	return p, p.ValidateBasic()
}
