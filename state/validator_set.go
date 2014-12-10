package state

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/tendermint/tendermint/merkle"
)

//-------------------------------------
// Implements sort for sorting validators by id.

type ValidatorSlice []*Validator

func (vs ValidatorSlice) Len() int {
	return len(vs)
}

func (vs ValidatorSlice) Less(i, j int) bool {
	return bytes.Compare(vs[i].Address, vs[j].Address) == -1
}

func (vs ValidatorSlice) Swap(i, j int) {
	it := vs[i]
	vs[i] = vs[j]
	vs[j] = it
}

//-------------------------------------

// ValidatorSet represent a set of *Validator at a given height.
// The validators can be fetched by address or index.
// The index is in order of .Address, so the index are the same
// for all rounds of a given blockchain height.
// On the other hand, the .AccumPower of each validator and
// the designated .Proposer() of a set changes every round,
// upon calling .IncrementAccum().
// NOTE: Not goroutine-safe.
// NOTE: All get/set to validators should copy the value for safety.
// TODO: consider validator Accum overflow
// TODO: replace validators []*Validator with github.com/jaekwon/go-ibbs?
type ValidatorSet struct {
	validators []*Validator

	// cache
	proposer         *Validator
	totalVotingPower uint64
}

func NewValidatorSet(vals []*Validator) *ValidatorSet {
	validators := make([]*Validator, len(vals))
	for i, val := range vals {
		validators[i] = val.Copy()
	}
	sort.Sort(ValidatorSlice(validators))
	return &ValidatorSet{
		validators: validators,
	}
}

func (valSet *ValidatorSet) IncrementAccum() {
	// Decrement from previous proposer
	oldProposer := valSet.Proposer()
	oldProposer.Accum -= int64(valSet.TotalVotingPower())
	valSet.Update(oldProposer)
	var newProposer *Validator
	// Increment accum and find new proposer
	// NOTE: updates validators in place.
	for _, val := range valSet.validators {
		val.Accum += int64(val.VotingPower)
		newProposer = newProposer.CompareAccum(val)
	}
	valSet.proposer = newProposer
}

func (valSet *ValidatorSet) Copy() *ValidatorSet {
	validators := make([]*Validator, len(valSet.validators))
	for i, val := range valSet.validators {
		// NOTE: must copy, since IncrementAccum updates in place.
		validators[i] = val.Copy()
	}
	return &ValidatorSet{
		validators:       validators,
		proposer:         valSet.proposer,
		totalVotingPower: valSet.totalVotingPower,
	}
}

func (valSet *ValidatorSet) HasAddress(address []byte) bool {
	idx := sort.Search(len(valSet.validators), func(i int) bool {
		return bytes.Compare(address, valSet.validators[i].Address) <= 0
	})
	return idx != len(valSet.validators) && bytes.Compare(valSet.validators[idx].Address, address) == 0
}

func (valSet *ValidatorSet) GetByAddress(address []byte) (index uint, val *Validator) {
	idx := sort.Search(len(valSet.validators), func(i int) bool {
		return bytes.Compare(address, valSet.validators[i].Address) <= 0
	})
	if idx != len(valSet.validators) && bytes.Compare(valSet.validators[idx].Address, address) == 0 {
		return uint(idx), valSet.validators[idx].Copy()
	} else {
		return 0, nil
	}
}

func (valSet *ValidatorSet) GetByIndex(index uint) (address []byte, val *Validator) {
	val = valSet.validators[index]
	return val.Address, val.Copy()
}

func (valSet *ValidatorSet) Size() uint {
	return uint(len(valSet.validators))
}

func (valSet *ValidatorSet) TotalVotingPower() uint64 {
	if valSet.totalVotingPower == 0 {
		for _, val := range valSet.validators {
			valSet.totalVotingPower += val.VotingPower
		}
	}
	return valSet.totalVotingPower
}

func (valSet *ValidatorSet) Proposer() (proposer *Validator) {
	if valSet.proposer == nil {
		for _, val := range valSet.validators {
			valSet.proposer = valSet.proposer.CompareAccum(val)
		}
	}
	return valSet.proposer.Copy()
}

func (valSet *ValidatorSet) Hash() []byte {
	if len(valSet.validators) == 0 {
		return nil
	}
	hashables := make([]merkle.Hashable, len(valSet.validators))
	for i, val := range valSet.validators {
		hashables[i] = val
	}
	return merkle.HashFromHashables(hashables)
}

func (valSet *ValidatorSet) Add(val *Validator) (added bool) {
	val = val.Copy()
	idx := sort.Search(len(valSet.validators), func(i int) bool {
		return bytes.Compare(val.Address, valSet.validators[i].Address) <= 0
	})
	if idx == len(valSet.validators) {
		valSet.validators = append(valSet.validators, val)
		// Invalidate cache
		valSet.proposer = nil
		valSet.totalVotingPower = 0
		return true
	} else if bytes.Compare(valSet.validators[idx].Address, val.Address) == 0 {
		return false
	} else {
		newValidators := append(valSet.validators[:idx], val)
		newValidators = append(newValidators, valSet.validators[idx:]...)
		valSet.validators = newValidators
		// Invalidate cache
		valSet.proposer = nil
		valSet.totalVotingPower = 0
		return true
	}
}

func (valSet *ValidatorSet) Update(val *Validator) (updated bool) {
	index, sameVal := valSet.GetByAddress(val.Address)
	if sameVal == nil {
		return false
	} else {
		valSet.validators[index] = val.Copy()
		// Invalidate cache
		valSet.proposer = nil
		valSet.totalVotingPower = 0
		return true
	}
}

func (valSet *ValidatorSet) Remove(address []byte) (val *Validator, removed bool) {
	idx := sort.Search(len(valSet.validators), func(i int) bool {
		return bytes.Compare(address, valSet.validators[i].Address) <= 0
	})
	if idx == len(valSet.validators) || bytes.Compare(valSet.validators[idx].Address, address) != 0 {
		return nil, false
	} else {
		removedVal := valSet.validators[idx]
		newValidators := valSet.validators[:idx]
		if idx+1 < len(valSet.validators) {
			newValidators = append(newValidators, valSet.validators[idx+1:]...)
		}
		valSet.validators = newValidators
		// Invalidate cache
		valSet.proposer = nil
		valSet.totalVotingPower = 0
		return removedVal, true
	}
}

func (valSet *ValidatorSet) Iterate(fn func(index uint, val *Validator) bool) {
	for i, val := range valSet.validators {
		stop := fn(uint(i), val.Copy())
		if stop {
			break
		}
	}
}

func (valSet *ValidatorSet) String() string {
	return valSet.StringWithIndent("")
}

func (valSet *ValidatorSet) StringWithIndent(indent string) string {
	valStrings := []string{}
	valSet.Iterate(func(index uint, val *Validator) bool {
		valStrings = append(valStrings, val.String())
		return false
	})
	return fmt.Sprintf(`ValidatorSet{
%s  Proposer: %v
%s  Validators:
%s    %v
%s}`,
		indent, valSet.Proposer().String(),
		indent,
		indent, strings.Join(valStrings, "\n"+indent+"    "),
		indent)

}
