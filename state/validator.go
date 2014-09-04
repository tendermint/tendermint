package state

import (
	"io"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

// Holds state for a Validator at a given height+round.
// Meant to be discarded every round of the consensus protocol.
// TODO consider moving this to another common types package.
type Validator struct {
	Account
	BondHeight  uint32
	VotingPower uint64
	Accum       int64
}

// Used to persist the state of ConsensusStateControl.
func ReadValidator(r io.Reader, n *int64, err *error) *Validator {
	return &Validator{
		Account: Account{
			Id:     ReadUInt64(r, n, err),
			PubKey: ReadByteSlice(r, n, err),
		},
		BondHeight:  ReadUInt32(r, n, err),
		VotingPower: ReadUInt64(r, n, err),
		Accum:       ReadInt64(r, n, err),
	}
}

// Creates a new copy of the validator so we can mutate accum.
func (v *Validator) Copy() *Validator {
	return &Validator{
		Account:     v.Account,
		BondHeight:  v.BondHeight,
		VotingPower: v.VotingPower,
		Accum:       v.Accum,
	}
}

// Used to persist the state of ConsensusStateControl.
func (v *Validator) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt64(w, v.Id, &n, &err)
	WriteByteSlice(w, v.PubKey, &n, &err)
	WriteUInt32(w, v.BondHeight, &n, &err)
	WriteUInt64(w, v.VotingPower, &n, &err)
	WriteInt64(w, v.Accum, &n, &err)
	return
}

//-----------------------------------------------------------------------------

// Not goroutine-safe.
type ValidatorSet struct {
	validators map[uint64]*Validator
}

func NewValidatorSet(validators map[uint64]*Validator) *ValidatorSet {
	if validators == nil {
		validators = make(map[uint64]*Validator)
	}
	return &ValidatorSet{
		validators: validators,
	}
}

func (v *ValidatorSet) IncrementAccum() {
	totalDelta := int64(0)
	for _, validator := range v.validators {
		validator.Accum += int64(validator.VotingPower)
		totalDelta += int64(validator.VotingPower)
	}
	proposer := v.GetProposer()
	proposer.Accum -= totalDelta
	// NOTE: sum(v) here should be zero.
	if true {
		totalAccum := int64(0)
		for _, validator := range v.validators {
			totalAccum += validator.Accum
		}
		if totalAccum != 0 {
			Panicf("Total Accum of validators did not equal 0. Got: ", totalAccum)
		}
	}
}

func (v *ValidatorSet) Copy() *ValidatorSet {
	mapCopy := map[uint64]*Validator{}
	for _, val := range v.validators {
		mapCopy[val.Id] = val.Copy()
	}
	return &ValidatorSet{
		validators: mapCopy,
	}
}

func (v *ValidatorSet) Add(validator *Validator) {
	v.validators[validator.Id] = validator
}

func (v *ValidatorSet) Get(id uint64) *Validator {
	return v.validators[id]
}

func (v *ValidatorSet) Map() map[uint64]*Validator {
	return v.validators
}

func (v *ValidatorSet) Size() int {
	return len(v.validators)
}

// TODO: cache proposer. invalidate upon increment.
func (v *ValidatorSet) GetProposer() (proposer *Validator) {
	highestAccum := int64(0)
	for _, validator := range v.validators {
		if validator.Accum > highestAccum {
			highestAccum = validator.Accum
			proposer = validator
		} else if validator.Accum == highestAccum {
			if validator.Id < proposer.Id { // Seniority
				proposer = validator
			}
		}
	}
	return
}
