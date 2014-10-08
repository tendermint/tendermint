package blocks

import (
	"errors"
	"io"

	. "github.com/tendermint/tendermint/binary"
)

const (
	VoteTypeBare      = byte(0x00)
	VoteTypePrecommit = byte(0x01)
	VoteTypeCommit    = byte(0x02)
)

var (
	ErrVoteUnexpectedPhase      = errors.New("Unexpected phase")
	ErrVoteInvalidAccount       = errors.New("Invalid round vote account")
	ErrVoteInvalidSignature     = errors.New("Invalid round vote signature")
	ErrVoteInvalidBlockHash     = errors.New("Invalid block hash")
	ErrVoteConflictingSignature = errors.New("Conflicting round vote signature")
)

// Represents a bare, precommit, or commit vote for proposals.
type Vote struct {
	Height    uint32
	Round     uint16
	Type      byte
	BlockHash []byte // empty if vote is nil.
	Signature
}

func ReadVote(r io.Reader, n *int64, err *error) *Vote {
	return &Vote{
		Height:    ReadUInt32(r, n, err),
		Round:     ReadUInt16(r, n, err),
		Type:      ReadByte(r, n, err),
		BlockHash: ReadByteSlice(r, n, err),
		Signature: ReadSignature(r, n, err),
	}
}

func (v *Vote) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt32(w, v.Height, &n, &err)
	WriteUInt16(w, v.Round, &n, &err)
	WriteByte(w, v.Type, &n, &err)
	WriteByteSlice(w, v.BlockHash, &n, &err)
	WriteBinary(w, v.Signature, &n, &err)
	return
}

func (v *Vote) GenDocument() []byte {
	oldSig := v.Signature
	v.Signature = Signature{}
	doc := BinaryBytes(v)
	v.Signature = oldSig
	return doc
}

func (v *Vote) GetSignature() Signature {
	return v.Signature
}

func (v *Vote) SetSignature(sig Signature) {
	v.Signature = sig
}
