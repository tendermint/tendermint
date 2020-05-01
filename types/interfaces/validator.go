package interfaces

import (
	tmmath "github.com/tendermint/tendermint/libs/math"
)

type Validator interface {
	CompareProposerPriority(other *Validator) *Validator
	ValidatorListString(vals []*Validator)
	Bytes() []byte
}

type ValidatorSet interface {
	CopyIncrementProposerPriority(int32) *ValidatorSet
	IncrementProposerPriority(times int32)
	incrementProposerPriority() *Validator
	RescalePriorities(diffMax int64)
	Copy() *ValidatorSet
	HasAddress([]byte)
	GetByAddress([]byte) (uint32, *Validator, bool)
	GetByIndex(uint32) ([]byte, *Validator)
	TotalVotingPower() int64
	GetProposer() *Validator
	Hash() []byte
	VerifyCommitTrusting(string, BlockID, int64, *Commit, tmmath.Fraction) error
}
