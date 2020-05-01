package interfaces

import (
	tmbits "github.com/tendermint/tendermint/libs/bits"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// Block needs to be threadsafe
type Block interface {
	ValidateBasic() error
	Hash() tmbytes.HexBytes
	MakePartSet(uint32) *PartSet
}

type CommitSig interface {
	ValidateBasic() error
	ForBlock() bool
	NewCommitSigAbsent() CommitSig
	Absent() bool
	BlockID(BlockID) BlockID
}

type Commit interface {
	ValidateBasic() error
	CommitToVoteSet(string, *Commit, *ValidatorSet) *VoteSet
	GetVote(uint32) *Vote
	GetHeight() int64
	GetRound() int32
	Type() byte
	BitArray() *tmbits.BitArray
	VoteSignBytes(string, uint32) []byte
	GetByIndex(uint32) *Vote
	IsCommit() bool
	Hash() tmbytes.HexBytes
}

type BlockID interface {
	ValidateBasic() error
	Key() string
	Equals(other BlockID) bool
	IsZero() bool
	IsComplete() bool
	GetHash() []byte
}
