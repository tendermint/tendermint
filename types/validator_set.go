package types

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/pkg/errors"
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
func (valSet *ValidatorSet) IncrementAccum(times int) {
	// Add VotingPower * times to each validator and order into heap.
	validatorsHeap := cmn.NewHeap()
	for _, val := range valSet.Validators {
		// check for overflow both multiplication and sum
		val.Accum = safeAddClip(val.Accum, safeMulClip(val.VotingPower, int64(times)))
		validatorsHeap.PushComparable(val, accumComparable{val})
	}

	// Decrement the validator with most accum times times
	for i := 0; i < times; i++ {
		mostest := validatorsHeap.Peek().(*Validator)
		if i == times-1 {
			valSet.Proposer = mostest
		}

		// mind underflow
		mostest.Accum = safeSubClip(mostest.Accum, valSet.TotalVotingPower())
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

// HasAddress returns true if address given is in the validator set, false -
// otherwise.
func (valSet *ValidatorSet) HasAddress(address []byte) bool {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address, valSet.Validators[i].Address) <= 0
	})
	return idx < len(valSet.Validators) && bytes.Equal(valSet.Validators[idx].Address, address)
}

// GetByAddress returns an index of the validator with address and validator
// itself if found. Otherwise, -1 and nil are returned.
func (valSet *ValidatorSet) GetByAddress(address []byte) (index int, val *Validator) {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address, valSet.Validators[i].Address) <= 0
	})
	if idx < len(valSet.Validators) && bytes.Equal(valSet.Validators[idx].Address, address) {
		return idx, valSet.Validators[idx].Copy()
	}
	return -1, nil
}

// GetByIndex returns the validator's address and validator itself by index.
// It returns nil values if index is less than 0 or greater or equal to
// len(ValidatorSet.Validators).
func (valSet *ValidatorSet) GetByIndex(index int) (address []byte, val *Validator) {
	if index < 0 || index >= len(valSet.Validators) {
		return nil, nil
	}
	val = valSet.Validators[index]
	return val.Address, val.Copy()
}

// Size returns the length of the validator set.
func (valSet *ValidatorSet) Size() int {
	return len(valSet.Validators)
}

// TotalVotingPower returns the sum of the voting powers of all validators.
func (valSet *ValidatorSet) TotalVotingPower() int64 {
	if valSet.totalVotingPower == 0 {
		for _, val := range valSet.Validators {
			// mind overflow
			valSet.totalVotingPower = safeAddClip(valSet.totalVotingPower, val.VotingPower)
		}
	}
	return valSet.totalVotingPower
}

// GetProposer returns the current proposer. If the validator set is empty, nil
// is returned.
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

// Hash returns the Merkle root hash build using validators (as leaves) in the
// set.
func (valSet *ValidatorSet) Hash() []byte {
	if len(valSet.Validators) == 0 {
		return nil
	}
	hashers := make([]merkle.Hasher, len(valSet.Validators))
	for i, val := range valSet.Validators {
		hashers[i] = val
	}
	return merkle.SimpleHashFromHashers(hashers)
}

// Add adds val to the validator set and returns true. It returns false if val
// is already in the set.
func (valSet *ValidatorSet) Add(val *Validator) (added bool) {
	val = val.Copy()
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(val.Address, valSet.Validators[i].Address) <= 0
	})
	if idx >= len(valSet.Validators) {
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

// Update updates val and returns true. It returns false if val is not present
// in the set.
func (valSet *ValidatorSet) Update(val *Validator) (updated bool) {
	index, sameVal := valSet.GetByAddress(val.Address)
	if sameVal == nil {
		return false
	}
	valSet.Validators[index] = val.Copy()
	// Invalidate cache
	valSet.Proposer = nil
	valSet.totalVotingPower = 0
	return true
}

// Remove deletes the validator with address. It returns the validator removed
// and true. If returns nil and false if validator is not present in the set.
func (valSet *ValidatorSet) Remove(address []byte) (val *Validator, removed bool) {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address, valSet.Validators[i].Address) <= 0
	})
	if idx >= len(valSet.Validators) || !bytes.Equal(valSet.Validators[idx].Address, address) {
		return nil, false
	}
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

// Iterate will run the given function over the set.
func (valSet *ValidatorSet) Iterate(fn func(index int, val *Validator) bool) {
	for i, val := range valSet.Validators {
		stop := fn(i, val.Copy())
		if stop {
			break
		}
	}
}

// Verify that +2/3 of the set had signed the given signBytes
func (valSet *ValidatorSet) VerifyCommit(chainID string, blockID BlockID, height int64, commit *Commit) error {
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
		precommitSignBytes := precommit.SignBytes(chainID)
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
	}
	return fmt.Errorf("Invalid commit -- insufficient voting power: got %v, needed %v",
		talliedVotingPower, (valSet.TotalVotingPower()*2/3 + 1))
}

// VerifyCommitAny will check to see if the set would
// be valid with a different validator set.
//
// valSet is the validator set that we know
// * over 2/3 of the power in old signed this block
//
// newSet is the validator set that signed this block
// * only votes from old are sufficient for 2/3 majority
//   in the new set as well
//
// That means that:
// * 10% of the valset can't just declare themselves kings
// * If the validator set is 3x old size, we need more proof to trust
func (valSet *ValidatorSet) VerifyCommitAny(newSet *ValidatorSet, chainID string,
	blockID BlockID, height int64, commit *Commit) error {

	if newSet.Size() != len(commit.Precommits) {
		return errors.Errorf("Invalid commit -- wrong set size: %v vs %v", newSet.Size(), len(commit.Precommits))
	}
	if height != commit.Height() {
		return errors.Errorf("Invalid commit -- wrong height: %v vs %v", height, commit.Height())
	}

	oldVotingPower := int64(0)
	newVotingPower := int64(0)
	seen := map[int]bool{}
	round := commit.Round()

	for idx, precommit := range commit.Precommits {
		// first check as in VerifyCommit
		if precommit == nil {
			continue
		}
		if precommit.Height != height {
			// return certerr.ErrHeightMismatch(height, precommit.Height)
			return errors.Errorf("Blocks don't match - %d vs %d", round, precommit.Round)
		}
		if precommit.Round != round {
			return errors.Errorf("Invalid commit -- wrong round: %v vs %v", round, precommit.Round)
		}
		if precommit.Type != VoteTypePrecommit {
			return errors.Errorf("Invalid commit -- not precommit @ index %v", idx)
		}
		if !blockID.Equals(precommit.BlockID) {
			continue // Not an error, but doesn't count
		}

		// we only grab by address, ignoring unknown validators
		vi, ov := valSet.GetByAddress(precommit.ValidatorAddress)
		if ov == nil || seen[vi] {
			continue // missing or double vote...
		}
		seen[vi] = true

		// Validate signature old school
		precommitSignBytes := precommit.SignBytes(chainID)
		if !ov.PubKey.VerifyBytes(precommitSignBytes, precommit.Signature) {
			return errors.Errorf("Invalid commit -- invalid signature: %v", precommit)
		}
		// Good precommit!
		oldVotingPower += ov.VotingPower

		// check new school
		_, cv := newSet.GetByIndex(idx)
		if cv.PubKey.Equals(ov.PubKey) {
			// make sure this is properly set in the current block as well
			newVotingPower += cv.VotingPower
		}
	}

	if oldVotingPower <= valSet.TotalVotingPower()*2/3 {
		return errors.Errorf("Invalid commit -- insufficient old voting power: got %v, needed %v",
			oldVotingPower, (valSet.TotalVotingPower()*2/3 + 1))
	} else if newVotingPower <= newSet.TotalVotingPower()*2/3 {
		return errors.Errorf("Invalid commit -- insufficient cur voting power: got %v, needed %v",
			newVotingPower, (newSet.TotalVotingPower()*2/3 + 1))
	}
	return nil
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

// RandValidatorSet returns a randomized validator set, useful for testing.
// NOTE: PrivValidator are in order.
// UNSTABLE
func RandValidatorSet(numValidators int, votingPower int64) (*ValidatorSet, []*PrivValidatorFS) {
	vals := make([]*Validator, numValidators)
	privValidators := make([]*PrivValidatorFS, numValidators)
	for i := 0; i < numValidators; i++ {
		val, privValidator := RandValidator(false, votingPower)
		vals[i] = val
		privValidators[i] = privValidator
	}
	valSet := NewValidatorSet(vals)
	sort.Sort(PrivValidatorsByAddress(privValidators))
	return valSet, privValidators
}

///////////////////////////////////////////////////////////////////////////////
// Safe multiplication and addition/subtraction

func safeMul(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, false
	}
	if a == 1 {
		return b, false
	}
	if b == 1 {
		return a, false
	}
	if a == math.MinInt64 || b == math.MinInt64 {
		return -1, true
	}
	c := a * b
	return c, c/b != a
}

func safeAdd(a, b int64) (int64, bool) {
	if b > 0 && a > math.MaxInt64-b {
		return -1, true
	} else if b < 0 && a < math.MinInt64-b {
		return -1, true
	}
	return a + b, false
}

func safeSub(a, b int64) (int64, bool) {
	if b > 0 && a < math.MinInt64+b {
		return -1, true
	} else if b < 0 && a > math.MaxInt64+b {
		return -1, true
	}
	return a - b, false
}

func safeMulClip(a, b int64) int64 {
	c, overflow := safeMul(a, b)
	if overflow {
		if (a < 0 || b < 0) && !(a < 0 && b < 0) {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return c
}

func safeAddClip(a, b int64) int64 {
	c, overflow := safeAdd(a, b)
	if overflow {
		if b < 0 {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return c
}

func safeSubClip(a, b int64) int64 {
	c, overflow := safeSub(a, b)
	if overflow {
		if b > 0 {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return c
}
