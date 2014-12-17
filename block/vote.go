package block

import (
	"errors"
	"fmt"
	"io"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

const (
	VoteTypePrevote   = byte(0x00)
	VoteTypePrecommit = byte(0x01)
	VoteTypeCommit    = byte(0x02)
)

var (
	ErrVoteUnexpectedStep       = errors.New("Unexpected step")
	ErrVoteInvalidAccount       = errors.New("Invalid round vote account")
	ErrVoteInvalidSignature     = errors.New("Invalid round vote signature")
	ErrVoteInvalidBlockHash     = errors.New("Invalid block hash")
	ErrVoteConflictingSignature = errors.New("Conflicting round vote signature")
)

// Represents a prevote, precommit, or commit vote for proposals from validators.
// Commit votes get aggregated into the next block's Validaiton.
// See the whitepaper for details.
type Vote struct {
	Height     uint
	Round      uint
	Type       byte
	BlockHash  []byte        // empty if vote is nil.
	BlockParts PartSetHeader // zero if vote is nil.
	Signature  SignatureEd25519
}

func (vote *Vote) WriteSignBytes(w io.Writer, n *int64, err *error) {
	WriteUVarInt(vote.Height, w, n, err)
	WriteUVarInt(vote.Round, w, n, err)
	WriteByte(vote.Type, w, n, err)
	WriteByteSlice(vote.BlockHash, w, n, err)
	WriteBinary(vote.BlockParts, w, n, err)
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
