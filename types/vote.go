package types

import (
	"errors"
	"fmt"
	"io"

	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/wire"
)

var (
	ErrVoteUnexpectedStep   = errors.New("Unexpected step")
	ErrVoteInvalidAccount   = errors.New("Invalid round vote account")
	ErrVoteInvalidSignature = errors.New("Invalid round vote signature")
	ErrVoteInvalidBlockHash = errors.New("Invalid block hash")
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
	Height           int                  `json:"height"`
	Round            int                  `json:"round"`
	Type             byte                 `json:"type"`
	BlockHash        []byte               `json:"block_hash"`         // empty if vote is nil.
	BlockPartsHeader PartSetHeader        `json:"block_parts_header"` // zero if vote is nil.
	Signature        acm.SignatureEd25519 `json:"signature"`
}

// Types of votes
const (
	VoteTypePrevote   = byte(0x01)
	VoteTypePrecommit = byte(0x02)
)

func (vote *Vote) WriteSignBytes(chainID string, w io.Writer, n *int64, err *error) {
	wire.WriteTo([]byte(Fmt(`{"chain_id":"%s","vote":`, chainID)), w, n, err)
	vote.writeSignBytes(w, n, err)
	wire.WriteTo([]byte(`}`), w, n, err)
}

// makes for a cleaner DupeoutTx.WriteSignBytes
func (vote *Vote) writeSignBytes(w io.Writer, n *int64, err *error) {
	wire.WriteTo([]byte(Fmt(`{"block_hash":"%X","block_parts_header":%v`, vote.BlockHash, vote.BlockPartsHeader)), w, n, err)
	wire.WriteTo([]byte(Fmt(`,"height":%v,"round":%v,"type":%v}`, vote.Height, vote.Round, vote.Type)), w, n, err)
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

	return fmt.Sprintf("Vote{%v/%02d/%v(%v) %X#%v %v}", vote.Height, vote.Round, vote.Type, typeString, Fingerprint(vote.BlockHash), vote.BlockPartsHeader, vote.Signature)
}

//--------------------------------------------------------------------------------
// TODO: Move blocks/Validation to here?

// Common interface between *consensus.VoteSet and types.Validation
type VoteSetReader interface {
	Height() int
	Round() int
	Type() byte
	Size() int
	BitArray() *BitArray
	GetByIndex(int) *Vote
	IsCommit() bool
}
