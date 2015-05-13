package types

import (
	"errors"
	"fmt"
	"io"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/config"
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
// Commit votes get aggregated into the next block's Validaiton.
// See the whitepaper for details.
type Vote struct {
	Height     uint                     `json:"height"`
	Round      uint                     `json:"round"`
	Type       byte                     `json:"type"`
	BlockHash  []byte                   `json:"block_hash"`  // empty if vote is nil.
	BlockParts PartSetHeader            `json:"block_parts"` // zero if vote is nil.
	Signature  account.SignatureEd25519 `json:"signature"`
}

// Types of votes
const (
	VoteTypePrevote   = byte(0x01)
	VoteTypePrecommit = byte(0x02)
	VoteTypeCommit    = byte(0x03)
)

func (vote *Vote) WriteSignBytes(w io.Writer, n *int64, err *error) {
	// We hex encode the network name so we don't deal with escaping issues.
	binary.WriteTo([]byte(Fmt(`{"network":"%X"`, config.App().GetString("network"))), w, n, err)
	binary.WriteTo([]byte(Fmt(`,"vote":{"block_hash":"%X","block_parts":%v`, vote.BlockHash, vote.BlockParts)), w, n, err)
	binary.WriteTo([]byte(Fmt(`,"height":%v,"round":%v,"type":%v}}`, vote.Height, vote.Round, vote.Type)), w, n, err)
}

func (vote *Vote) Copy() *Vote {
	voteCopy := *vote
	return &voteCopy
}

func (vote *Vote) String() string {
	var typeString string
	switch vote.Type {
	case VoteTypePrevote:
		typeString = "Prevote"
	case VoteTypePrecommit:
		typeString = "Precommit"
	case VoteTypeCommit:
		typeString = "Commit"
	default:
		panic("Unknown vote type")
	}

	return fmt.Sprintf("Vote{%v/%02d/%v(%v) %X#%v %v}", vote.Height, vote.Round, vote.Type, typeString, Fingerprint(vote.BlockHash), vote.BlockParts, vote.Signature)
}
