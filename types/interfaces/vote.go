package interfaces

import (
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/bits"
)

type Vote interface {
	ValidateBasic() error
	CommitSig() CommitSig
	SignBytes(string) []byte
	Verify(string, crypto.PubKey) error
}

// Should be thread safe
type VoteSet interface {
	AddVote(*Vote) (bool, error)
	SetPeerMaj23(string, BlockID) error
	MakeCommit() *Commit
}

type VoteSetReader interface {
	GetHeight() int64
	GetRound() int32
	Type() byte
	Size() int
	BitArray() *bits.BitArray
	GetByIndex(uint32) *Vote
	IsCommit() bool
}
