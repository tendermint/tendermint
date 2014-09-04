package state

import (
	"io"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	//. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
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
func ReadValidator(r io.Reader) *Validator {
	return &Validator{
		Account: Account{
			Id:     Readuint64(r),
			PubKey: ReadByteSlice(r),
		},
		BondHeight:  Readuint32(r),
		VotingPower: Readuint64(r),
		Accum:       Readint64(r),
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
	n, err = WriteTo(UInt64(v.Id), w, n, err)
	n, err = WriteTo(v.PubKey, w, n, err)
	n, err = WriteTo(UInt32(v.BondHeight), w, n, err)
	n, err = WriteTo(UInt64(v.VotingPower), w, n, err)
	n, err = WriteTo(Int64(v.Accum), w, n, err)
	return
}

//-----------------------------------------------------------------------------

// TODO: Ensure that double signing never happens via an external persistent check.
type PrivValidator struct {
	PrivAccount
	db *db_.LevelDB
}

// Modifies the vote object in memory.
// Double signing results in an error.
func (pv *PrivValidator) SignVote(vote *Vote) error {
	return nil
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
		valdiators: validators,
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
	for _, val := range validators {
		mapCopy[val.Id] = val.Copy()
	}
	return &ValidatorSet{
		validators: mapCopy,
	}
}

func (v *ValidatorSet) Add(validator *Valdaitor) {
	v.validators[validator.Id] = validator
}

func (v *ValidatorSet) Get(id uint64) *Validator {
	return v.validators[validator.Id]
}

func (v *ValidatorSet) Map() map[uint64]*Validator {
	return v.validators
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
