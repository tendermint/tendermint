package types

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
)

const (
	// MaxVoteBytes is a maximum vote size (including amino overhead).
	MaxVoteBytes int64 = 223
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

func NewConflictingVoteError(val *Validator, voteA, voteB *Vote) *ErrVoteConflictingVotes {
	return &ErrVoteConflictingVotes{
		&DuplicateVoteEvidence{
			PubKey: val.PubKey,
			VoteA:  voteA,
			VoteB:  voteB,
		},
	}
}

// Address is hex bytes.
type Address = crypto.Address

// Vote represents a prevote, precommit, or commit vote from validators for
// consensus.
type Vote struct {
	Type             SignedMsgType `json:"type"`
	Height           int64         `json:"height"`
	Round            int           `json:"round"`
	Timestamp        time.Time     `json:"timestamp"`
	BlockID          BlockID       `json:"block_id"` // zero if vote is nil.
	ValidatorAddress Address       `json:"validator_address"`
	ValidatorIndex   int           `json:"validator_index"`
	Signature        []byte        `json:"signature"`
}

func (vote *Vote) SignBytes(chainID string) []byte {
	bz, err := cdc.MarshalBinaryLengthPrefixed(CanonicalizeVote(chainID, vote))
	if err != nil {
		panic(err)
	}
	return bz
}

func (vote *Vote) Copy() *Vote {
	voteCopy := *vote
	return &voteCopy
}

func (vote *Vote) String() string {
	if vote == nil {
		return "nil-Vote"
	}
	var typeString string
	switch vote.Type {
	case PrevoteType:
		typeString = "Prevote"
	case PrecommitType:
		typeString = "Precommit"
	default:
		cmn.PanicSanity("Unknown vote type")
	}

	return fmt.Sprintf("Vote{%v:%X %v/%02d/%v(%v) %X %X @ %s}",
		vote.ValidatorIndex,
		cmn.Fingerprint(vote.ValidatorAddress),
		vote.Height,
		vote.Round,
		vote.Type,
		typeString,
		cmn.Fingerprint(vote.BlockID.Hash),
		cmn.Fingerprint(vote.Signature),
		CanonicalTime(vote.Timestamp),
	)
}

func (vote *Vote) Verify(chainID string, pubKey crypto.PubKey) error {
	if !bytes.Equal(pubKey.Address(), vote.ValidatorAddress) {
		return ErrVoteInvalidValidatorAddress
	}

	if !pubKey.VerifyBytes(vote.SignBytes(chainID), vote.Signature) {
		return ErrVoteInvalidSignature
	}
	return nil
}

// ValidateBasic performs basic validation.
func (vote *Vote) ValidateBasic() error {
	if !IsVoteTypeValid(vote.Type) {
		return errors.New("Invalid Type")
	}
	if vote.Height < 0 {
		return errors.New("Negative Height")
	}
	if vote.Round < 0 {
		return errors.New("Negative Round")
	}

	// NOTE: Timestamp validation is subtle and handled elsewhere.

	if err := vote.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("Wrong BlockID: %v", err)
	}
	if len(vote.ValidatorAddress) != crypto.AddressSize {
		return fmt.Errorf("Expected ValidatorAddress size to be %d bytes, got %d bytes",
			crypto.AddressSize,
			len(vote.ValidatorAddress),
		)
	}
	if vote.ValidatorIndex < 0 {
		return errors.New("Negative ValidatorIndex")
	}
	if len(vote.Signature) == 0 {
		return errors.New("Signature is missing")
	}
	if len(vote.Signature) > MaxSignatureSize {
		return fmt.Errorf("Signature is too big (max: %d)", MaxSignatureSize)
	}
	return nil
}
