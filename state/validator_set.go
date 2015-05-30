package state

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/merkle"
	"github.com/tendermint/tendermint/types"
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
	totalVotingPower uint64
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
func (valSet *ValidatorSet) IncrementAccum(times uint) {
	// Add VotingPower * times to each validator and order into heap.
	validatorsHeap := NewHeap()
	for _, val := range valSet.Validators {
		val.Accum += int64(val.VotingPower) * int64(times) // TODO: mind overflow
		validatorsHeap.Push(val, accumComparable(val.Accum))
	}

	// Decrement the validator with most accum, times times.
	for i := uint(0); i < times; i++ {
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

func (valSet *ValidatorSet) GetByAddress(address []byte) (index uint, val *Validator) {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address, valSet.Validators[i].Address) <= 0
	})
	if idx != len(valSet.Validators) && bytes.Compare(valSet.Validators[idx].Address, address) == 0 {
		return uint(idx), valSet.Validators[idx].Copy()
	} else {
		return 0, nil
	}
}

func (valSet *ValidatorSet) GetByIndex(index uint) (address []byte, val *Validator) {
	val = valSet.Validators[index]
	return val.Address, val.Copy()
}

func (valSet *ValidatorSet) Size() uint {
	return uint(len(valSet.Validators))
}

func (valSet *ValidatorSet) TotalVotingPower() uint64 {
	if valSet.totalVotingPower == 0 {
		for _, val := range valSet.Validators {
			valSet.totalVotingPower += val.VotingPower
		}
	}
	return valSet.totalVotingPower
}

func (valSet *ValidatorSet) Proposer() (proposer *Validator) {
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
	return merkle.HashFromHashables(hashables)
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

func (valSet *ValidatorSet) Iterate(fn func(index uint, val *Validator) bool) {
	for i, val := range valSet.Validators {
		stop := fn(uint(i), val.Copy())
		if stop {
			break
		}
	}
}

// Verify that +2/3 of the set had signed the given signBytes
func (valSet *ValidatorSet) VerifyValidation(chainID string, hash []byte, parts types.PartSetHeader, height uint, v *types.Validation) error {
	if valSet.Size() != uint(len(v.Commits)) {
		return errors.New(Fmt("Invalid validation -- wrong set size: %v vs %v",
			valSet.Size(), len(v.Commits)))
	}

	talliedVotingPower := uint64(0)
	seenValidators := map[string]struct{}{}

	for idx, commit := range v.Commits {
		// may be zero, in which case skip.
		if commit.Signature.IsZero() {
			continue
		}
		_, val := valSet.GetByIndex(uint(idx))
		commitSignBytes := account.SignBytes(chainID, &types.Vote{
			Height: height, Round: commit.Round, Type: types.VoteTypeCommit,
			BlockHash:  hash,
			BlockParts: parts,
		})

		// Validate
		if _, seen := seenValidators[string(val.Address)]; seen {
			return fmt.Errorf("Duplicate validator for commit %v for Validation %v", commit, v)
		}

		if !val.PubKey.VerifyBytes(commitSignBytes, commit.Signature) {
			return fmt.Errorf("Invalid signature for commit %v for Validation %v", commit, v)
		}

		// Tally
		seenValidators[string(val.Address)] = struct{}{}
		talliedVotingPower += val.VotingPower
	}

	if talliedVotingPower > valSet.TotalVotingPower()*2/3 {
		return nil
	} else {
		return fmt.Errorf("insufficient voting power %v, needed %v",
			talliedVotingPower, (valSet.TotalVotingPower()*2/3 + 1))
	}
}

func (valSet *ValidatorSet) String() string {
	return valSet.StringIndented("")
}

func (valSet *ValidatorSet) StringIndented(indent string) string {
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

type accumComparable uint64

// We want to find the validator with the greatest accum.
func (ac accumComparable) Less(o interface{}) bool {
	return uint64(ac) < uint64(o.(accumComparable))
}
