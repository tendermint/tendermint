package types

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
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

// Represents a prevote, precommit, or commit vote from validators for consensus.
type Vote struct {
	ValidatorAddress data.Bytes       `json:"validator_address"`
	ValidatorIndex   int              `json:"validator_index"`
	Height           int64            `json:"height"`
	Round            int              `json:"round"`
	Timestamp        time.Time        `json:"timestamp"`
	Type             byte             `json:"type"`
	BlockID          BlockID          `json:"block_id"` // zero if vote is nil.
	Signature        crypto.Signature `json:"signature"`
}

func (vote *Vote) WriteSignBytes(chainID string, w io.Writer, n *int, err *error) {
	wire.WriteJSON(CanonicalJSONOnceVote{
		chainID,
		CanonicalVote(vote),
	}, w, n, err)
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
	case VoteTypePrevote:
		typeString = "Prevote"
	case VoteTypePrecommit:
		typeString = "Precommit"
	default:
		cmn.PanicSanity("Unknown vote type")
	}

	return fmt.Sprintf("Vote{%v:%X %v/%02d/%v(%v) %X %v @ %s}",
		vote.ValidatorIndex, cmn.Fingerprint(vote.ValidatorAddress),
		vote.Height, vote.Round, vote.Type, typeString,
		cmn.Fingerprint(vote.BlockID.Hash), vote.Signature,
		CanonicalTime(vote.Timestamp))
}

func (vote *Vote) Verify(chainID string, pubKey crypto.PubKey) error {
	if !bytes.Equal(pubKey.Address(), vote.ValidatorAddress) {
		return ErrVoteInvalidValidatorAddress
	}

	if !pubKey.VerifyBytes(SignBytes(chainID, vote), vote.Signature) {
		return ErrVoteInvalidSignature
	}
	return nil
}
