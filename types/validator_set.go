package types

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/merkle"
)

// ValidatorSet represent a set of *Validator at a given height.
// The validators can be fetched by address or index.
// The index is in order of .Address, so the indices are fixed
// for all rounds of a given blockchain height.
// On the other hand, the .AccumPower of each validator and
// the designated .GetProposer() of a set changes every round,
// upon calling .IncrementAccum().
// NOTE: Not goroutine-safe.
// NOTE: All get/set to validators should copy the value for safety.
// TODO: consider validator Accum overflow
type ValidatorSet struct {
	// NOTE: persisted via reflect, must be exported.
	Validators []*Validator `json:"validators"`
	Proposer   *Validator   `json:"proposer"`

	// cached (unexported)
	totalVotingPower int64
}

func NewValidatorSet(vals []*Validator) *ValidatorSet {
	validators := make([]*Validator, len(vals))
	for i, val := range vals {
		validators[i] = val.Copy()
	}
	sort.Sort(ValidatorsByAddress(validators))
	vs := &ValidatorSet{
		Validators: validators,
	}

	if vals != nil {
		vs.IncrementAccum(1)
	}

	return vs
}

// incrementAccum and update the proposer
// TODO: mind the overflow when times and votingPower shares too large.
func (valSet *ValidatorSet) IncrementAccum(times int) {
	// Add VotingPower * times to each validator and order into heap.
	validatorsHeap := cmn.NewHeap()
	for _, val := range valSet.Validators {
		val.Accum += int64(val.VotingPower) * int64(times) // TODO: mind overflow
		validatorsHeap.Push(val, accumComparable{val})
	}

	// Decrement the validator with most accum times times
	for i := 0; i < times; i++ {
		mostest := validatorsHeap.Peek().(*Validator)
		if i == times-1 {
			valSet.Proposer = mostest
		}
		mostest.Accum -= int64(valSet.TotalVotingPower())
		validatorsHeap.Update(mostest, accumComparable{mostest})
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
		Proposer:         valSet.Proposer,
		totalVotingPower: valSet.totalVotingPower,
	}
}

func (valSet *ValidatorSet) HasAddress(address []byte) bool {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address, valSet.Validators[i].Address) <= 0
	})
	return idx != len(valSet.Validators) && bytes.Equal(valSet.Validators[idx].Address, address)
}

func (valSet *ValidatorSet) GetByAddress(address []byte) (index int, val *Validator) {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address, valSet.Validators[i].Address) <= 0
	})
	if idx != len(valSet.Validators) && bytes.Equal(valSet.Validators[idx].Address, address) {
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

func (valSet *ValidatorSet) GetProposer() (proposer *Validator) {
	if len(valSet.Validators) == 0 {
		return nil
	}
	if valSet.Proposer == nil {
		valSet.Proposer = valSet.findProposer()
	}
	return valSet.Proposer.Copy()
}

func (valSet *ValidatorSet) findProposer() *Validator {
	var proposer *Validator
	for _, val := range valSet.Validators {
		if proposer == nil || !bytes.Equal(val.Address, proposer.Address) {
			proposer = proposer.CompareAccum(val)
		}
	}
	return proposer
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
		valSet.Proposer = nil
		valSet.totalVotingPower = 0
		return true
	} else if bytes.Equal(valSet.Validators[idx].Address, val.Address) {
		return false
	} else {
		newValidators := make([]*Validator, len(valSet.Validators)+1)
		copy(newValidators[:idx], valSet.Validators[:idx])
		newValidators[idx] = val
		copy(newValidators[idx+1:], valSet.Validators[idx:])
		valSet.Validators = newValidators
		// Invalidate cache
		valSet.Proposer = nil
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
		valSet.Proposer = nil
		valSet.totalVotingPower = 0
		return true
	}
}

func (valSet *ValidatorSet) Remove(address []byte) (val *Validator, removed bool) {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address, valSet.Validators[i].Address) <= 0
	})
	if idx == len(valSet.Validators) || !bytes.Equal(valSet.Validators[idx].Address, address) {
		return nil, false
	} else {
		removedVal := valSet.Validators[idx]
		newValidators := valSet.Validators[:idx]
		if idx+1 < len(valSet.Validators) {
			newValidators = append(newValidators, valSet.Validators[idx+1:]...)
		}
		valSet.Validators = newValidators
		// Invalidate cache
		valSet.Proposer = nil
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
func (valSet *ValidatorSet) VerifyCommit(chainID string, blockID BlockID, height int, commit *Commit) error {
	if valSet.Size() != len(commit.Precommits) {
		return fmt.Errorf("Invalid commit -- wrong set size: %v vs %v", valSet.Size(), len(commit.Precommits))
	}
	if height != commit.Height() {
		return fmt.Errorf("Invalid commit -- wrong height: %v vs %v", height, commit.Height())
	}

	talliedVotingPower := int64(0)
	round := commit.Round()

	for idx, precommit := range commit.Precommits {
		// may be nil if validator skipped.
		if precommit == nil {
			continue
		}
		if precommit.Height != height {
			return fmt.Errorf("Invalid commit -- wrong height: %v vs %v", height, precommit.Height)
		}
		if precommit.Round != round {
			return fmt.Errorf("Invalid commit -- wrong round: %v vs %v", round, precommit.Round)
		}
		if precommit.Type != VoteTypePrecommit {
			return fmt.Errorf("Invalid commit -- not precommit @ index %v", idx)
		}
		_, val := valSet.GetByIndex(idx)
		// Validate signature
		precommitSignBytes := SignBytes(chainID, precommit)
		if !val.PubKey.VerifyBytes(precommitSignBytes, precommit.Signature) {
			return fmt.Errorf("Invalid commit -- invalid signature: %v", precommit)
		}
		if !blockID.Equals(precommit.BlockID) {
			continue // Not an error, but doesn't count
		}
		// Good precommit!
		talliedVotingPower += val.VotingPower
	}

	if talliedVotingPower > valSet.TotalVotingPower()*2/3 {
		return nil
	} else {
		return fmt.Errorf("Invalid commit -- insufficient voting power: got %v, needed %v",
			talliedVotingPower, (valSet.TotalVotingPower()*2/3 + 1))
	}
}

// VerifyCommitAny verifies that +2/3 of this set had signed the given signBytes.
// Unlike VerifyCommit(), this function can verify commits with different sets.
// VerifyCommitAny verifies that more than 2/3 of the old validator set have
// signed this block, as well as whether more than 2/3 of the new validator set
// have signed this block.
func (valSet *ValidatorSet) VerifyCommitAny(newVals *ValidatorSet, chainID string, blockID BlockID, height int, commit *Commit) error {
	if newVals.Size() != len(commit.Precommits) {
		return fmt.Errorf("Invalid commit -- wrong set size: %v vs %v", newVals.Size(), len(commit.Precommits))
	}
	if height != commit.Height() {
		return fmt.Errorf("Invalid commit -- wrong height: %v vs %v", height, commit.Height())
	}

	curVotingPower := int64(0)
	newVotingPower := int64(0)
	seen := map[int]bool{}
	round := commit.Round()

	for idx, precommit := range commit.Precommits {
		// may be nil if validator skipped.
		if precommit == nil {
			continue
		}
		if precommit.Height != height {
			return fmt.Errorf("Invalid commit -- wrong height: %v vs %v", height, precommit.Height)
		}
		if precommit.Round != round {
			return fmt.Errorf("Invalid commit -- wrong round: %v vs %v", round, precommit.Round)
		}
		if precommit.Type != VoteTypePrecommit {
			return fmt.Errorf("Invalid commit -- not precommit @ index %v", idx)
		}
		if !blockID.Equals(precommit.BlockID) {
			continue // Not an error, but doesn't count
		}

		// check whether a current validator has signed this precommit
		valIdx, val := valSet.GetByAddress(precommit.ValidatorAddress)
		// if val is nill then the current validator hasn't signed the precommit
		if val == nil || seen[valIdx] {
			continue // missing or double vote
		}
		seen[valIdx] = true

		precommitSignBytes := SignBytes(chainID, precommit)

		// check that the validator from the current set has signed the precommit
		if !val.PubKey.VerifyBytes(precommitSignBytes, precommit.Signature) {
			return fmt.Errorf("Invalid commit -- invalid signature: %v", precommit)
		}
		curVotingPower += val.VotingPower

		// check that the validator from the new set has signed the precommit
		_, newVal := newVals.GetByIndex(idx)
		if newVal.PubKey.Equals(val.PubKey) {
			newVotingPower += newVal.VotingPower
		}

	}

	if curVotingPower <= valSet.TotalVotingPower()*2/3 {
		return fmt.Errorf("Invalid commit -- insufficient current voting power: got %v, needed %v", curVotingPower, (valSet.TotalVotingPower()*2/3 + 1))
	} else if newVotingPower <= newVals.TotalVotingPower()*2/3 {
		return fmt.Errorf("Invalid commit -- insufficient new voting power: got %v, needed %v", newVotingPower, (newVals.TotalVotingPower()*2/3 + 1))
	}

	return nil
}

func (valSet *ValidatorSet) ToBytes() []byte {
	buf, n, err := new(bytes.Buffer), new(int), new(error)
	wire.WriteBinary(valSet, buf, n, err)
	if *err != nil {
		cmn.PanicCrisis(*err)
	}
	return buf.Bytes()
}

func (valSet *ValidatorSet) FromBytes(b []byte) {
	r, n, err := bytes.NewReader(b), new(int), new(error)
	wire.ReadBinary(valSet, r, 0, n, err)
	if *err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		cmn.PanicCrisis(*err)
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
		indent, valSet.GetProposer().String(),
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

type accumComparable struct {
	*Validator
}

// We want to find the validator with the greatest accum.
func (ac accumComparable) Less(o interface{}) bool {
	other := o.(accumComparable).Validator
	larger := ac.CompareAccum(other)
	return bytes.Equal(larger.Address, ac.Address)
}

//----------------------------------------
// For testing

// NOTE: PrivValidator are in order.
func RandValidatorSet(numValidators int, votingPower int64) (*ValidatorSet, []*PrivValidator) {
	vals := make([]*Validator, numValidators)
	privValidators := make([]*PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		val, privValidator := RandValidator(false, votingPower)
		vals[i] = val
		privValidators[i] = privValidator
	}
	valSet := NewValidatorSet(vals)
	sort.Sort(PrivValidatorsByAddress(privValidators))
	return valSet, privValidators
}
