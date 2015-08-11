package types

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/merkle"
)

// ValidatorSet represent a set of *Validator at a given height.
// The validators can be fetched by address or index.
// The index is in order of .Address, so the indices are fixed
// for all rounds of a given blockchain height.
// On the other hand, the .AccumPower of each validator and
// the designated .Proposer() of a set changes every round,
// upon calling .IncrementAccum().
// NOTE: Not goroutine-safe.
// NOTE: All get/set to validators should copy the value for safety.
// TODO: consider validator Accum overflow
// TODO: replace validators []*Validator with github.com/jaekwon/go-ibbs?
type ValidatorSet struct {
	Validators []*Validator // NOTE: persisted via reflect, must be exported.

	// cached (unexported)
	proposer         *Validator
	totalVotingPower int64
}

func NewValidatorSet(vals []*Validator) *ValidatorSet {
	validators := make([]*Validator, len(vals))
	for i, val := range vals {
		validators[i] = val.Copy()
	}
	sort.Sort(ValidatorsByAddress(validators))
	return &ValidatorSet{
		Validators: validators,
	}
}

// TODO: mind the overflow when times and votingPower shares too large.
func (valSet *ValidatorSet) IncrementAccum(times int) {
	// Add VotingPower * times to each validator and order into heap.
	validatorsHeap := NewHeap()
	for _, val := range valSet.Validators {
		val.Accum += int64(val.VotingPower) * int64(times) // TODO: mind overflow
		validatorsHeap.Push(val, accumComparable(val.Accum))
	}

	// Decrement the validator with most accum, times times.
	for i := 0; i < times; i++ {
		mostest := validatorsHeap.Peek().(*Validator)
		if i == times-1 {
			valSet.proposer = mostest
		}
		mostest.Accum -= int64(valSet.TotalVotingPower())
		validatorsHeap.Update(mostest, accumComparable(mostest.Accum))
	}
}

func (valSet *ValidatorSet) Copy() *ValidatorSet {
	validators := make([]*Validator, len(valSet.Validators))
	for i, val := range valSet.Validators {
		// NOTE: must copy, since IncrementAccum updates in place.
		validators[i] = val.Copy()
	}
	return &ValidatorSet{
		Validators:       validators,
		proposer:         valSet.proposer,
		totalVotingPower: valSet.totalVotingPower,
	}
}

func (valSet *ValidatorSet) HasAddress(address []byte) bool {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address, valSet.Validators[i].Address) <= 0
	})
	return idx != len(valSet.Validators) && bytes.Compare(valSet.Validators[idx].Address, address) == 0
}

func (valSet *ValidatorSet) GetByAddress(address []byte) (index int, val *Validator) {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address, valSet.Validators[i].Address) <= 0
	})
	if idx != len(valSet.Validators) && bytes.Compare(valSet.Validators[idx].Address, address) == 0 {
		return idx, valSet.Validators[idx].Copy()
	} else {
		return 0, nil
	}
}

func (valSet *ValidatorSet) GetByIndex(index int) (address []byte, val *Validator) {
	val = valSet.Validators[index]
	return val.Address, val.Copy()
}

func (valSet *ValidatorSet) Size() int {
	return len(valSet.Validators)
}

func (valSet *ValidatorSet) TotalVotingPower() int64 {
	if valSet.totalVotingPower == 0 {
		for _, val := range valSet.Validators {
			valSet.totalVotingPower += val.VotingPower
		}
	}
	return valSet.totalVotingPower
}

func (valSet *ValidatorSet) Proposer() (proposer *Validator) {
	if len(valSet.Validators) == 0 {
		return nil
	}
	if valSet.proposer == nil {
		for _, val := range valSet.Validators {
			valSet.proposer = valSet.proposer.CompareAccum(val)
		}
	}
	return valSet.proposer.Copy()
}

func (valSet *ValidatorSet) Hash() []byte {
	if len(valSet.Validators) == 0 {
		return nil
	}
	hashables := make([]merkle.Hashable, len(valSet.Validators))
	for i, val := range valSet.Validators {
		hashables[i] = val
	}
	return merkle.SimpleHashFromHashables(hashables)
}

func (valSet *ValidatorSet) Add(val *Validator) (added bool) {
	val = val.Copy()
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(val.Address, valSet.Validators[i].Address) <= 0
	})
	if idx == len(valSet.Validators) {
		valSet.Validators = append(valSet.Validators, val)
		// Invalidate cache
		valSet.proposer = nil
		valSet.totalVotingPower = 0
		return true
	} else if bytes.Compare(valSet.Validators[idx].Address, val.Address) == 0 {
		return false
	} else {
		newValidators := make([]*Validator, len(valSet.Validators)+1)
		copy(newValidators[:idx], valSet.Validators[:idx])
		newValidators[idx] = val
		copy(newValidators[idx+1:], valSet.Validators[idx:])
		valSet.Validators = newValidators
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
		valSet.Validators[index] = val.Copy()
		// Invalidate cache
		valSet.proposer = nil
		valSet.totalVotingPower = 0
		return true
	}
}

func (valSet *ValidatorSet) Remove(address []byte) (val *Validator, removed bool) {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address, valSet.Validators[i].Address) <= 0
	})
	if idx == len(valSet.Validators) || bytes.Compare(valSet.Validators[idx].Address, address) != 0 {
		return nil, false
	} else {
		removedVal := valSet.Validators[idx]
		newValidators := valSet.Validators[:idx]
		if idx+1 < len(valSet.Validators) {
			newValidators = append(newValidators, valSet.Validators[idx+1:]...)
		}
		valSet.Validators = newValidators
		// Invalidate cache
		valSet.proposer = nil
		valSet.totalVotingPower = 0
		return removedVal, true
	}
}

func (valSet *ValidatorSet) Iterate(fn func(index int, val *Validator) bool) {
	for i, val := range valSet.Validators {
		stop := fn(i, val.Copy())
		if stop {
			break
		}
	}
}

// Verify that +2/3 of the set had signed the given signBytes
func (valSet *ValidatorSet) VerifyValidation(chainID string,
	hash []byte, parts PartSetHeader, height int, v *Validation) error {
	if valSet.Size() != len(v.Precommits) {
		return fmt.Errorf("Invalid validation -- wrong set size: %v vs %v", valSet.Size(), len(v.Precommits))
	}
	if height != v.Height() {
		return fmt.Errorf("Invalid validation -- wrong height: %v vs %v", height, v.Height())
	}

	talliedVotingPower := int64(0)
	round := v.Round()

	for idx, precommit := range v.Precommits {
		// may be nil if validator skipped.
		if precommit == nil {
			continue
		}
		if precommit.Height != height {
			return fmt.Errorf("Invalid validation -- wrong height: %v vs %v", height, precommit.Height)
		}
		if precommit.Round != round {
			return fmt.Errorf("Invalid validation -- wrong round: %v vs %v", round, precommit.Round)
		}
		if precommit.Type != VoteTypePrecommit {
			return fmt.Errorf("Invalid validation -- not precommit @ index %v", idx)
		}
		_, val := valSet.GetByIndex(idx)
		// Validate signature
		precommitSignBytes := account.SignBytes(chainID, precommit)
		if !val.PubKey.VerifyBytes(precommitSignBytes, precommit.Signature) {
			return fmt.Errorf("Invalid validation -- invalid signature: %v", precommit)
		}
		if !bytes.Equal(precommit.BlockHash, hash) {
			continue // Not an error, but doesn't count
		}
		if !parts.Equals(precommit.BlockParts) {
			continue // Not an error, but doesn't count
		}
		// Good precommit!
		talliedVotingPower += val.VotingPower
	}

	if talliedVotingPower > valSet.TotalVotingPower()*2/3 {
		return nil
	} else {
		return fmt.Errorf("Invalid validation -- insufficient voting power: got %v, needed %v",
			talliedVotingPower, (valSet.TotalVotingPower()*2/3 + 1))
	}
}

func (valSet *ValidatorSet) String() string {
	return valSet.StringIndented("")
}

func (valSet *ValidatorSet) StringIndented(indent string) string {
	if valSet == nil {
		return "nil-ValidatorSet"
	}
	valStrings := []string{}
	valSet.Iterate(func(index int, val *Validator) bool {
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

//-------------------------------------
// Implements sort for sorting validators by address.

type ValidatorsByAddress []*Validator

func (vs ValidatorsByAddress) Len() int {
	return len(vs)
}

func (vs ValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(vs[i].Address, vs[j].Address) == -1
}

func (vs ValidatorsByAddress) Swap(i, j int) {
	it := vs[i]
	vs[i] = vs[j]
	vs[j] = it
}

//-------------------------------------
// Use with Heap for sorting validators by accum

type accumComparable int64

// We want to find the validator with the greatest accum.
func (ac accumComparable) Less(o interface{}) bool {
	return int64(ac) > int64(o.(accumComparable))
}
