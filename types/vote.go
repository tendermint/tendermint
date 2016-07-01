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

type ErrVoteConflictingSignature struct {
	VoteA *Vote
	VoteB *Vote
}

func (err *ErrVoteConflictingSignature) Error() string {
	return "Conflicting round vote signature"
}

// Represents a prevote, precommit, or commit vote from validators for consensus.
type Vote struct {
	ValidatorAddress []byte                  `json:"validator_address"`
	ValidatorIndex   int                     `json:"validator_index"`
	Height           int                     `json:"height"`
	Round            int                     `json:"round"`
	Type             byte                    `json:"type"`
	BlockHash        []byte                  `json:"block_hash"`         // empty if vote is nil.
	BlockPartsHeader PartSetHeader           `json:"block_parts_header"` // zero if vote is nil.
	Signature        crypto.SignatureEd25519 `json:"signature"`
}

// Types of votes
const (
	VoteTypePrevote   = byte(0x01)
	VoteTypePrecommit = byte(0x02)
)

func (vote *Vote) WriteSignBytes(chainID string, w io.Writer, n *int, err *error) {
	wire.WriteTo([]byte(Fmt(`{"chain_id":"%s"`, chainID)), w, n, err)
	wire.WriteTo([]byte(Fmt(`,"vote":{"block_hash":"%X","block_parts_header":%v`, vote.BlockHash, vote.BlockPartsHeader)), w, n, err)
	wire.WriteTo([]byte(Fmt(`,"height":%v,"round":%v,"type":%v}}`, vote.Height, vote.Round, vote.Type)), w, n, err)
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
		Fingerprint(vote.BlockHash), vote.Signature)
}
