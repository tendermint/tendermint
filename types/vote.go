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
	ErrVoteUnexpectedStep          = errors.New("Unexpected step")
	ErrVoteInvalidValidatorIndex   = errors.New("Invalid round vote validator index")
	ErrVoteInvalidValidatorAddress = errors.New("Invalid round vote validator address")
	ErrVoteInvalidSignature        = errors.New("Invalid round vote signature")
	ErrVoteInvalidBlockHash        = errors.New("Invalid block hash")
)

type ErrVoteConflictingVotes struct {
	VoteA *Vote
	VoteB *Vote
}

func (err *ErrVoteConflictingVotes) Error() string {
	return "Conflicting votes"
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
	ValidatorAddress []byte                  `json:"validator_address"`
	ValidatorIndex   int                     `json:"validator_index"`
	Height           int                     `json:"height"`
	Round            int                     `json:"round"`
	Type             byte                    `json:"type"`
	BlockID          BlockID                 `json:"block_id"` // zero if vote is nil.
	Signature        crypto.SignatureEd25519 `json:"signature"`
}

func (vote *Vote) WriteSignBytes(chainID string, w io.Writer, n *int, err *error) {

	wire.WriteJSON(
		struct {
			ChainID string `json:"chain_id"`
			Vote    struct {
				BlockID          BlockID          `json:"block_id"`
				Height           int              `json:"height"`
				Round            int              `json:"round"`
				Signature        crypto.Signature `json:"signature"`
				Type             byte             `json:"type"`
				ValidatorAddress []byte           `json:"validator_address"`
				ValidatorIndex   int              `json:"validator_index"`
			} `json: "vote"`
		}{
			chainID,
			struct {
				BlockID          BlockID          `json:"block_id"`
				Height           int              `json:"height"`
				Round            int              `json:"round"`
				Signature        crypto.Signature `json:"signature"`
				Type             byte             `json:"type"`
				ValidatorAddress []byte           `json:"validator_address"`
				ValidatorIndex   int              `json:"validator_index"`
			}{
				vote.BlockID,
				vote.Height,
				vote.Round,
				vote.Signature,
				vote.Type,
				vote.ValidatorAddress,
				vote.ValidatorIndex,
			},
		},
		w, n, err)
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
		PanicSanity("Unknown vote type")
	}

	return fmt.Sprintf("Vote{%v:%X %v/%02d/%v(%v) %X %v}",
		vote.ValidatorIndex, Fingerprint(vote.ValidatorAddress),
		vote.Height, vote.Round, vote.Type, typeString,
		Fingerprint(vote.BlockID.Hash), vote.Signature)
}
