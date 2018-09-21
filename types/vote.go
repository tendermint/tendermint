package types

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	crypto "github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
)

const (
	// MaxVoteBytes is a maximum vote size (including amino overhead).
	MaxVoteBytes = 200
)

var (
	ErrVoteUnexpectedStep            = errors.New("Unexpected step")
	ErrVoteInvalidValidatorIndex     = errors.New("Invalid validator index")
	ErrVoteInvalidValidatorAddress   = errors.New("Invalid validator address")
	ErrVoteInvalidSignature          = errors.New("Invalid signature")
	ErrVoteInvalidBlockHash          = errors.New("Invalid block hash")
	ErrVoteNonDeterministicSignature = errors.New("Non-deterministic signature")
	ErrVoteNil                       = errors.New("Nil vote")
)

type ErrVoteConflictingVotes struct {
	*DuplicateVoteEvidence
}

func (err *ErrVoteConflictingVotes) Error() string {
	return fmt.Sprintf("Conflicting votes from validator %v", err.PubKey.Address())
}

func NewConflictingVoteError(val *Validator, voteA, voteB *SignedVote) *ErrVoteConflictingVotes {
	return &ErrVoteConflictingVotes{
		&DuplicateVoteEvidence{
			PubKey: val.PubKey,
			VoteA:  voteA,
			VoteB:  voteB,
		},
	}
}

// Types of votes
// TODO Make a new type "VoteType"
const (
	VoteTypePrevote   = byte(0x01)
	VoteTypePrecommit = byte(0x02)
)

func IsVoteTypeValid(type_ byte) bool {
	switch type_ {
	case VoteTypePrevote:
		return true
	case VoteTypePrecommit:
		return true
	default:
		return false
	}
}

// Address is hex bytes. TODO: crypto.Address
type Address = cmn.HexBytes

// Represents a prevote, precommit, or commit vote from validators for consensus.
type UnsignedVote struct {
	ValidatorAddress Address   `json:"validator_address"`
	ValidatorIndex   int       `json:"validator_index"`
	Height           int64     `json:"height"`
	Round            int       `json:"round"`
	Timestamp        time.Time `json:"timestamp"`
	Type             byte      `json:"type"`
	BlockID          BlockID   `json:"block_id"` // zero if vote is nil.
	ChainID          string    `json:"chain_id"`
}

type SignedVote struct {
	Vote      *UnsignedVote
	Signature []byte
}

type Error struct {
	ErrCode     uint16
	Description string
}

func (vote *UnsignedVote) SignBytes() []byte {
	bz, err := cdc.MarshalBinary(vote)
	if err != nil {
		panic(err)
	}
	return bz
}

func (vote *UnsignedVote) Copy() *UnsignedVote {
	voteCopy := *vote
	return &voteCopy
}

func (vote *UnsignedVote) String() string {
	if vote == nil {
		return "nil-UnsignedVote"
	}
	var typeString string
	switch vote.Type {
	case VoteTypePrevote:
		typeString = "Prevote"
	case VoteTypePrecommit:
		typeString = "Precommit"
	default:
		cmn.PanicSanity("Unknown vote type")
	}

	return fmt.Sprintf("UnsignedVote{%v:%X %v/%02d/%v(%v) %X @ %s}",
		vote.ValidatorIndex, cmn.Fingerprint(vote.ValidatorAddress),
		vote.Height, vote.Round, vote.Type, typeString,
		cmn.Fingerprint(vote.BlockID.Hash),
		// TODO(ismail): add corresponding
		//cmn.Fingerprint(vote.Signature),
		CanonicalTime(vote.Timestamp))
}

func (vote *SignedVote) Verify(pubKey crypto.PubKey) error {
	if vote == nil || vote.Vote == nil {
		return errors.New("called verify on nil/empty SignedVote")
	}
	if !bytes.Equal(pubKey.Address(), vote.Vote.ValidatorAddress) {
		return ErrVoteInvalidValidatorAddress
	}

	if !pubKey.VerifyBytes(vote.Vote.SignBytes(), vote.Signature) {
		return ErrVoteInvalidSignature
	}
	return nil
}
