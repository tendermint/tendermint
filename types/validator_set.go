package types

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/tendermint/tendermint/crypto/merkle"
	cmn "github.com/tendermint/tendermint/libs/common"
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

func NewValidatorSet(valz []*Validator) *ValidatorSet {
	if valz != nil && len(valz) == 0 {
		panic("validator set initialization slice cannot be an empty slice (but it can be nil)")
	}
	validators := make([]*Validator, len(valz))
	for i, val := range valz {
		validators[i] = val.Copy()
	}
	sort.Sort(ValidatorsByAddress(validators))
	vals := &ValidatorSet{
		Validators: validators,
	}
	if len(valz) > 0 {
		vals.IncrementAccum(1)
	}

	return vals
}

// Nil or empty validator sets are invalid.
func (vals *ValidatorSet) IsNilOrEmpty() bool {
	return vals == nil || len(vals.Validators) == 0
}

// Increment Accum and update the proposer on a copy, and return it.
func (vals *ValidatorSet) CopyIncrementAccum(times int) *ValidatorSet {
	copy := vals.Copy()
	copy.IncrementAccum(times)
	return copy
}

// IncrementAccum increments accum of each validator and updates the
// proposer. Panics if validator set is empty.
func (vals *ValidatorSet) IncrementAccum(times int) {

	// Add VotingPower * times to each validator and order into heap.
	validatorsHeap := cmn.NewHeap()
	for _, val := range vals.Validators {
		// Check for overflow both multiplication and sum.
		val.Accum = safeAddClip(val.Accum, safeMulClip(val.VotingPower, int64(times)))
		validatorsHeap.PushComparable(val, accumComparable{val})
	}

	// Decrement the validator with most accum times times.
	for i := 0; i < times; i++ {
		mostest := validatorsHeap.Peek().(*Validator)
		// mind underflow
		mostest.Accum = safeSubClip(mostest.Accum, vals.TotalVotingPower())

		if i == times-1 {
			vals.Proposer = mostest
		} else {
			validatorsHeap.Update(mostest, accumComparable{mostest})
		}
	}
}

// Copy each validator into a new ValidatorSet
func (vals *ValidatorSet) Copy() *ValidatorSet {
	validators := make([]*Validator, len(vals.Validators))
	for i, val := range vals.Validators {
		// NOTE: must copy, since IncrementAccum updates in place.
		validators[i] = val.Copy()
	}
	return &ValidatorSet{
		Validators:       validators,
		Proposer:         vals.Proposer,
		totalVotingPower: vals.totalVotingPower,
	}
}

// HasAddress returns true if address given is in the validator set, false -
// otherwise.
func (vals *ValidatorSet) HasAddress(address []byte) bool {
	idx := sort.Search(len(vals.Validators), func(i int) bool {
		return bytes.Compare(address, vals.Validators[i].Address) <= 0
	})
	return idx < len(vals.Validators) && bytes.Equal(vals.Validators[idx].Address, address)
}

// GetByAddress returns an index of the validator with address and validator
// itself if found. Otherwise, -1 and nil are returned.
func (vals *ValidatorSet) GetByAddress(address []byte) (index int, val *Validator) {
	idx := sort.Search(len(vals.Validators), func(i int) bool {
		return bytes.Compare(address, vals.Validators[i].Address) <= 0
	})
	if idx < len(vals.Validators) && bytes.Equal(vals.Validators[idx].Address, address) {
		return idx, vals.Validators[idx].Copy()
	}
	return -1, nil
}

// GetByIndex returns the validator's address and validator itself by index.
// It returns nil values if index is less than 0 or greater or equal to
// len(ValidatorSet.Validators).
func (vals *ValidatorSet) GetByIndex(index int) (address []byte, val *Validator) {
	if index < 0 || index >= len(vals.Validators) {
		return nil, nil
	}
	val = vals.Validators[index]
	return val.Address, val.Copy()
}

// Size returns the length of the validator set.
func (vals *ValidatorSet) Size() int {
	return len(vals.Validators)
}

// TotalVotingPower returns the sum of the voting powers of all validators.
func (vals *ValidatorSet) TotalVotingPower() int64 {
	if vals.totalVotingPower == 0 {
		for _, val := range vals.Validators {
			// mind overflow
			vals.totalVotingPower = safeAddClip(vals.totalVotingPower, val.VotingPower)
		}
	}
	return vals.totalVotingPower
}

// GetProposer returns the current proposer. If the validator set is empty, nil
// is returned.
func (vals *ValidatorSet) GetProposer() (proposer *Validator) {
	if len(vals.Validators) == 0 {
		return nil
	}
	if vals.Proposer == nil {
		vals.Proposer = vals.findProposer()
	}
	return vals.Proposer.Copy()
}

func (vals *ValidatorSet) findProposer() *Validator {
	var proposer *Validator
	for _, val := range vals.Validators {
		if proposer == nil || !bytes.Equal(val.Address, proposer.Address) {
			proposer = proposer.CompareAccum(val)
		}
	}
	return proposer
}

// Hash returns the Merkle root hash build using validators (as leaves) in the
// set.
func (vals *ValidatorSet) Hash() []byte {
	if len(vals.Validators) == 0 {
		return nil
	}
	hashers := make([]merkle.Hasher, len(vals.Validators))
	for i, val := range vals.Validators {
		hashers[i] = val
	}
	return merkle.SimpleHashFromHashers(hashers)
}

// Add adds val to the validator set and returns true. It returns false if val
// is already in the set.
func (vals *ValidatorSet) Add(val *Validator) (added bool) {
	val = val.Copy()
	idx := sort.Search(len(vals.Validators), func(i int) bool {
		return bytes.Compare(val.Address, vals.Validators[i].Address) <= 0
	})
	if idx >= len(vals.Validators) {
		vals.Validators = append(vals.Validators, val)
		// Invalidate cache
		vals.Proposer = nil
		vals.totalVotingPower = 0
		return true
	} else if bytes.Equal(vals.Validators[idx].Address, val.Address) {
		return false
	} else {
		newValidators := make([]*Validator, len(vals.Validators)+1)
		copy(newValidators[:idx], vals.Validators[:idx])
		newValidators[idx] = val
		copy(newValidators[idx+1:], vals.Validators[idx:])
		vals.Validators = newValidators
		// Invalidate cache
		vals.Proposer = nil
		vals.totalVotingPower = 0
		return true
	}
}

// Update updates val and returns true. It returns false if val is not present
// in the set.
func (vals *ValidatorSet) Update(val *Validator) (updated bool) {
	index, sameVal := vals.GetByAddress(val.Address)
	if sameVal == nil {
		return false
	}
	vals.Validators[index] = val.Copy()
	// Invalidate cache
	vals.Proposer = nil
	vals.totalVotingPower = 0
	return true
}

// Remove deletes the validator with address. It returns the validator removed
// and true. If returns nil and false if validator is not present in the set.
func (vals *ValidatorSet) Remove(address []byte) (val *Validator, removed bool) {
	idx := sort.Search(len(vals.Validators), func(i int) bool {
		return bytes.Compare(address, vals.Validators[i].Address) <= 0
	})
	if idx >= len(vals.Validators) || !bytes.Equal(vals.Validators[idx].Address, address) {
		return nil, false
	}
	removedVal := vals.Validators[idx]
	newValidators := vals.Validators[:idx]
	if idx+1 < len(vals.Validators) {
		newValidators = append(newValidators, vals.Validators[idx+1:]...)
	}
	vals.Validators = newValidators
	// Invalidate cache
	vals.Proposer = nil
	vals.totalVotingPower = 0
	return removedVal, true
}

// Iterate will run the given function over the set.
func (vals *ValidatorSet) Iterate(fn func(index int, val *Validator) bool) {
	for i, val := range vals.Validators {
		stop := fn(i, val.Copy())
		if stop {
			break
		}
	}
}

// Verify that +2/3 of the set had signed the given signBytes.
func (vals *ValidatorSet) VerifyCommit(chainID string, blockID BlockID, height int64, commit *Commit) error {
	if vals.Size() != len(commit.Precommits) {
		return fmt.Errorf("Invalid commit -- wrong set size: %v vs %v", vals.Size(), len(commit.Precommits))
	}
	if height != commit.Height() {
		return fmt.Errorf("Invalid commit -- wrong height: %v vs %v", height, commit.Height())
	}
	if !blockID.Equals(commit.BlockID) {
		return fmt.Errorf("Invalid commit -- wrong block id: want %v got %v",
			blockID, commit.BlockID)
	}

	talliedVotingPower := int64(0)
	round := commit.Round()

	for idx, precommit := range commit.Precommits {
		if precommit == nil {
			continue // OK, some precommits can be missing.
		}
		if precommit.Height != height {
			return fmt.Errorf("Invalid commit -- wrong height: want %v got %v", height, precommit.Height)
		}
		if precommit.Round != round {
			return fmt.Errorf("Invalid commit -- wrong round: want %v got %v", round, precommit.Round)
		}
		if precommit.Type != VoteTypePrecommit {
			return fmt.Errorf("Invalid commit -- not precommit @ index %v", idx)
		}
		_, val := vals.GetByIndex(idx)
		// Validate signature.
		precommitSignBytes := precommit.SignBytes(chainID)
		if !val.PubKey.VerifyBytes(precommitSignBytes, precommit.Signature) {
			return fmt.Errorf("Invalid commit -- invalid signature: %v", precommit)
		}
		// Good precommit!
		if blockID.Equals(precommit.BlockID) {
			talliedVotingPower += val.VotingPower
		} else {
			// It's OK that the BlockID doesn't match.  We include stray
			// precommits to measure validator availability.
		}
	}

	if talliedVotingPower > vals.TotalVotingPower()*2/3 {
		return nil
	}
	return fmt.Errorf("Invalid commit -- insufficient voting power: got %v, needed %v",
		talliedVotingPower, (vals.TotalVotingPower()*2/3 + 1))
}

// VerifyFutureCommit will check to see if the set would be valid with a different
// validator set.
//
// vals is the old validator set that we know.  Over 2/3 of the power in old
// signed this block.
//
// In Tendermint, 1/3 of the voting power can halt or fork the chain, but 1/3
// can't make arbitrary state transitions.  You still need > 2/3 Byzantine to
// make arbitrary state transitions.
//
// To preserve this property in the light client, we also require > 2/3 of the
// old vals to sign the future commit at H, that way we preserve the property
// that if they weren't being truthful about the validator set at H (block hash
// -> vals hash) or about the app state (block hash -> app hash) we can slash
// > 2/3.  Otherwise, the lite client isn't providing the same security
// guarantees.
//
// Even if we added a slashing condition that if you sign a block header with
// the wrong validator set, then we would only need > 1/3 of signatures from
// the old vals on the new commit, it wouldn't be sufficient because the new
// vals can be arbitrary and commit some arbitrary app hash.
//
// newSet is the validator set that signed this block.  Only votes from new are
// sufficient for 2/3 majority in the new set as well, for it to be a valid
// commit.
//
// NOTE: This doesn't check whether the commit is a future commit, because the
// current height isn't part of the ValidatorSet.  Caller must check that the
// commit height is greater than the height for this validator set.
func (vals *ValidatorSet) VerifyFutureCommit(newSet *ValidatorSet, chainID string,
	blockID BlockID, height int64, commit *Commit) error {
	oldVals := vals

	// Commit must be a valid commit for newSet.
	err := newSet.VerifyCommit(chainID, blockID, height, commit)
	if err != nil {
		return err
	}

	// Check old voting power.
	oldVotingPower := int64(0)
	seen := map[int]bool{}
	round := commit.Round()

	for idx, precommit := range commit.Precommits {
		if precommit == nil {
			continue
		}
		if precommit.Height != height {
			return cmn.NewError("Blocks don't match - %d vs %d", round, precommit.Round)
		}
		if precommit.Round != round {
			return cmn.NewError("Invalid commit -- wrong round: %v vs %v", round, precommit.Round)
		}
		if precommit.Type != VoteTypePrecommit {
			return cmn.NewError("Invalid commit -- not precommit @ index %v", idx)
		}
		// See if this validator is in oldVals.
		idx, val := oldVals.GetByAddress(precommit.ValidatorAddress)
		if val == nil || seen[idx] {
			continue // missing or double vote...
		}
		seen[idx] = true

		// Validate signature.
		precommitSignBytes := precommit.SignBytes(chainID)
		if !val.PubKey.VerifyBytes(precommitSignBytes, precommit.Signature) {
			return cmn.NewError("Invalid commit -- invalid signature: %v", precommit)
		}
		// Good precommit!
		if blockID.Equals(precommit.BlockID) {
			oldVotingPower += val.VotingPower
		} else {
			// It's OK that the BlockID doesn't match.  We include stray
			// precommits to measure validator availability.
		}
	}

	if oldVotingPower <= oldVals.TotalVotingPower()*2/3 {
		return cmn.NewError("Invalid commit -- insufficient old voting power: got %v, needed %v",
			oldVotingPower, (oldVals.TotalVotingPower()*2/3 + 1))
	}
	return nil
}

func (vals *ValidatorSet) String() string {
	return vals.StringIndented("")
}

// String
func (vals *ValidatorSet) StringIndented(indent string) string {
	if vals == nil {
		return "nil-ValidatorSet"
	}
	valStrings := []string{}
	vals.Iterate(func(index int, val *Validator) bool {
		valStrings = append(valStrings, val.String())
		return false
	})
	return fmt.Sprintf(`ValidatorSet{
%s  Proposer: %v
%s  Validators:
%s    %v
%s}`,
		indent, vals.GetProposer().String(),
		indent,
		indent, strings.Join(valStrings, "\n"+indent+"    "),
		indent)

}

//-------------------------------------
// Implements sort for sorting validators by address.

// Sort validators by address
type ValidatorsByAddress []*Validator

func (valz ValidatorsByAddress) Len() int {
	return len(valz)
}

func (valz ValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(valz[i].Address, valz[j].Address) == -1
}

func (valz ValidatorsByAddress) Swap(i, j int) {
	it := valz[i]
	valz[i] = valz[j]
	valz[j] = it
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
func RandValidatorSet(numValidators int, votingPower int64) (*ValidatorSet, []PrivValidator) {
	valz := make([]*Validator, numValidators)
	privValidators := make([]PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		val, privValidator := RandValidator(false, votingPower)
		valz[i] = val
		privValidators[i] = privValidator
	}
	vals := NewValidatorSet(valz)
	sort.Sort(PrivValidatorsByAddress(privValidators))
	return vals, privValidators
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
