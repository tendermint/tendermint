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
	Height     uint
	Round      uint
	Type       byte
	BlockHash  []byte        // empty if vote is nil.
	BlockParts PartSetHeader // zero if vote is nil.
	Signature  account.SignatureEd25519
}

// Types of votes
const (
	VoteTypePrevote   = byte(0x01)
	VoteTypePrecommit = byte(0x02)
	VoteTypeCommit    = byte(0x03)
)

func (vote *Vote) WriteSignBytes(w io.Writer, n *int64, err *error) {
	// We hex encode the network name so we don't deal with escaping issues.
	binary.WriteTo([]byte(Fmt(`{"Network":"%X"`, config.App().GetString("Network"))), w, n, err)
	binary.WriteTo([]byte(Fmt(`,"Vote":{"BlockHash":"%X","BlockParts":%v`, vote.BlockHash, vote.BlockParts)), w, n, err)
	binary.WriteTo([]byte(Fmt(`,"Height":%v,"Round":%v,"Type":%v}}`, vote.Height, vote.Round, vote.Type)), w, n, err)
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

	return fmt.Sprintf("%v{%v/%v %X#%v %v}", typeString, vote.Height, vote.Round, Fingerprint(vote.BlockHash), vote.BlockParts, vote.Signature)
}
