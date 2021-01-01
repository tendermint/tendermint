package types

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"

	"github.com/tendermint/tendermint/crypto/merkle"
	tmmath "github.com/tendermint/tendermint/libs/math"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

const (
	// MaxTotalVotingPower - the maximum allowed total voting power.
	// It needs to be sufficiently small to, in all cases:
	// 1. prevent clipping in incrementProposerPriority()
	// 2. let (diff+diffMax-1) not overflow in IncrementProposerPriority()
	// (Proof of 1 is tricky, left to the reader).
	// It could be higher, but this is sufficiently large for our purposes,
	// and leaves room for defensive purposes.
	MaxTotalVotingPower = int64(math.MaxInt64) / 8

	DefaultDashVotingPower = int64(100)

	// PriorityWindowSizeFactor - is a constant that when multiplied with the
	// total voting power gives the maximum allowed distance between validator
	// priorities.
	PriorityWindowSizeFactor = 2
)

// ErrTotalVotingPowerOverflow is returned if the total voting power of the
// resulting validator set exceeds MaxTotalVotingPower.
var ErrTotalVotingPowerOverflow = fmt.Errorf("total voting power of resulting valset exceeds max %d",
	MaxTotalVotingPower)

// ValidatorSet represent a set of *Validator at a given height.
//
// The validators can be fetched by address or index.
// The index is in order of .VotingPower, so the indices are fixed for all
// rounds of a given blockchain height - ie. the validators are sorted by their
// voting power (descending). Secondary index - .ProTxHash (ascending).
//
// On the other hand, the .ProposerPriority of each validator and the
// designated .GetProposer() of a set changes every round, upon calling
// .IncrementProposerPriority().
//
// NOTE: Not goroutine-safe.
// NOTE: All get/set to validators should copy the value for safety.
type ValidatorSet struct {
	// NOTE: persisted via reflect, must be exported.
	Validators         []*Validator  `json:"validators"`
	Proposer           *Validator    `json:"proposer"`
	ThresholdPublicKey crypto.PubKey `json:"threshold_public_key"`

	// cached (unexported)
	totalVotingPower int64
}

// NewValidatorSet initializes a ValidatorSet by copying over the values from
// `valz`, a list of Validators. If valz is nil or empty, the new ValidatorSet
// will have an empty list of Validators.
//
// The addresses of validators in `valz` must be unique otherwise the function
// panics.
//
// Note the validator set size has an implied limit equal to that of the
// MaxVotesCount - commits by a validator set larger than this will fail
// validation.
func NewValidatorSet(valz []*Validator, newThresholdPublicKey crypto.PubKey) *ValidatorSet {
	vals := &ValidatorSet{}
	err := vals.updateWithChangeSet(valz, false, newThresholdPublicKey)
	if err != nil {
		panic(fmt.Sprintf("Cannot create validator set: %v", err))
	}
	if len(valz) > 0 {
		vals.IncrementProposerPriority(1)
	}
	return vals
}

func (vals *ValidatorSet) ValidateBasic() error {
	if vals.IsNilOrEmpty() {
		return errors.New("validator set is nil or empty")
	}

	for idx, val := range vals.Validators {
		if err := val.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid validator #%d: %w", idx, err)
		}
	}

	if err := vals.ThresholdPublicKeyValid(); err != nil {
		return fmt.Errorf("thresholdPublicKey error: %w", err)
	}

	if err := vals.Proposer.ValidateBasic(); err != nil {
		return fmt.Errorf("proposer failed validate basic, error: %w", err)
	}

	return nil
}

func (vals *ValidatorSet) Equals(other *ValidatorSet) bool {
	if !vals.ThresholdPublicKey.Equals(other.ThresholdPublicKey) {
		return false
	}
	if len(vals.Validators) != len(other.Validators) {
		return false
	}
	for i, val := range vals.Validators {
		if !bytes.Equal(val.Bytes(), other.Validators[i].Bytes()) {
			return false
		}
	}
	return true
}

// IsNilOrEmpty returns true if validator set is nil or empty.
func (vals *ValidatorSet) IsNilOrEmpty() bool {
	return vals == nil || len(vals.Validators) == 0
}

// IsNilOrEmpty returns true if validator set is nil or empty.
func (vals *ValidatorSet) ThresholdPublicKeyValid() error {
	if vals.ThresholdPublicKey == nil {
		return errors.New("threshold public key is not set")
	}
	if len(vals.ThresholdPublicKey.Bytes()) != bls12381.PubKeySize {
		return errors.New("threshold public key is wrong size")
	}
	if len(vals.Validators) == 1 {
		if !vals.Validators[0].PubKey.Equals(vals.ThresholdPublicKey) {
			return errors.New("incorrect threshold public key")
		}
	} else if len(vals.Validators) > 1 {
		recoveredThresholdPublicKey, err := bls12381.RecoverThresholdPublicKeyFromPublicKeys(vals.GetPublicKeys(),
			vals.GetProTxHashesAsByteArrays())
		if err != nil {
			return err
		} else if !recoveredThresholdPublicKey.Equals(vals.ThresholdPublicKey) {
			return errors.New("incorrect recovered threshold public key")
		}
	}
	return nil
}

// CopyIncrementProposerPriority increments ProposerPriority and updates the
// proposer on a copy, and returns it.
func (vals *ValidatorSet) CopyIncrementProposerPriority(times int32) *ValidatorSet {
	copy := vals.Copy()
	copy.IncrementProposerPriority(times)
	return copy
}

// IncrementProposerPriority increments ProposerPriority of each validator and
// updates the proposer. Panics if validator set is empty.
// `times` must be positive.
func (vals *ValidatorSet) IncrementProposerPriority(times int32) {
	if vals.IsNilOrEmpty() {
		panic("empty validator set")
	}
	if times <= 0 {
		panic("Cannot call IncrementProposerPriority with non-positive times")
	}

	// Cap the difference between priorities to be proportional to 2*totalPower by
	// re-normalizing priorities, i.e., rescale all priorities by multiplying with:
	//  2*totalVotingPower/(maxPriority - minPriority)
	diffMax := PriorityWindowSizeFactor * vals.TotalVotingPower()
	vals.RescalePriorities(diffMax)
	vals.shiftByAvgProposerPriority()

	var proposer *Validator
	// Call IncrementProposerPriority(1) times times.
	for i := int32(0); i < times; i++ {
		proposer = vals.incrementProposerPriority()
	}

	vals.Proposer = proposer
}

// RescalePriorities rescales the priorities such that the distance between the
// maximum and minimum is smaller than `diffMax`. Panics if validator set is
// empty.
func (vals *ValidatorSet) RescalePriorities(diffMax int64) {
	if vals.IsNilOrEmpty() {
		panic("empty validator set")
	}
	// NOTE: This check is merely a sanity check which could be
	// removed if all tests would init. voting power appropriately;
	// i.e. diffMax should always be > 0
	if diffMax <= 0 {
		return
	}

	// Calculating ceil(diff/diffMax):
	// Re-normalization is performed by dividing by an integer for simplicity.
	// NOTE: This may make debugging priority issues easier as well.
	diff := computeMaxMinPriorityDiff(vals)
	ratio := (diff + diffMax - 1) / diffMax
	if diff > diffMax && ratio != 0 {
		for _, val := range vals.Validators {
			val.ProposerPriority /= ratio
		}
	}
}

func (vals *ValidatorSet) incrementProposerPriority() *Validator {
	for _, val := range vals.Validators {
		// Check for overflow for sum.
		newPrio := safeAddClip(val.ProposerPriority, val.VotingPower)
		val.ProposerPriority = newPrio
	}
	// Decrement the validator with most ProposerPriority.
	mostest := vals.getValWithMostPriority()
	// Mind the underflow.
	mostest.ProposerPriority = safeSubClip(mostest.ProposerPriority, vals.TotalVotingPower())

	return mostest
}

// Should not be called on an empty validator set.
func (vals *ValidatorSet) computeAvgProposerPriority() int64 {
	n := int64(len(vals.Validators))
	sum := big.NewInt(0)
	for _, val := range vals.Validators {
		sum.Add(sum, big.NewInt(val.ProposerPriority))
	}
	avg := sum.Div(sum, big.NewInt(n))
	if avg.IsInt64() {
		return avg.Int64()
	}

	// This should never happen: each val.ProposerPriority is in bounds of int64.
	panic(fmt.Sprintf("Cannot represent avg ProposerPriority as an int64 %v", avg))
}

// Compute the difference between the max and min ProposerPriority of that set.
func computeMaxMinPriorityDiff(vals *ValidatorSet) int64 {
	if vals.IsNilOrEmpty() {
		panic("empty validator set")
	}
	max := int64(math.MinInt64)
	min := int64(math.MaxInt64)
	for _, v := range vals.Validators {
		if v.ProposerPriority < min {
			min = v.ProposerPriority
		}
		if v.ProposerPriority > max {
			max = v.ProposerPriority
		}
	}
	diff := max - min
	if diff < 0 {
		return -1 * diff
	}
	return diff
}

func (vals *ValidatorSet) getValWithMostPriority() *Validator {
	var res *Validator
	for _, val := range vals.Validators {
		res = res.CompareProposerPriority(val)
	}
	return res
}

func (vals *ValidatorSet) shiftByAvgProposerPriority() {
	if vals.IsNilOrEmpty() {
		panic("empty validator set")
	}
	avgProposerPriority := vals.computeAvgProposerPriority()
	for _, val := range vals.Validators {
		val.ProposerPriority = safeSubClip(val.ProposerPriority, avgProposerPriority)
	}
}

// Makes a copy of the validator list.
func validatorListCopy(valsList []*Validator) []*Validator {
	if valsList == nil {
		return nil
	}
	valsCopy := make([]*Validator, len(valsList))
	for i, val := range valsList {
		valsCopy[i] = val.Copy()
	}
	return valsCopy
}

// Copy each validator into a new ValidatorSet.
func (vals *ValidatorSet) Copy() *ValidatorSet {
	return &ValidatorSet{
		Validators:         validatorListCopy(vals.Validators),
		Proposer:           vals.Proposer,
		totalVotingPower:   vals.totalVotingPower,
		ThresholdPublicKey: vals.ThresholdPublicKey,
	}
}

// HasAddress returns true if address given is in the validator set, false -
// otherwise.
func (vals *ValidatorSet) HasProTxHash(proTxHash []byte) bool {
	for _, val := range vals.Validators {
		if bytes.Equal(val.ProTxHash, proTxHash) {
			return true
		}
	}
	return false
}

// HasAddress returns true if address given is in the validator set, false -
// otherwise.
func (vals *ValidatorSet) HasAddress(address []byte) bool {
	for _, val := range vals.Validators {
		if bytes.Equal(val.Address, address) {
			return true
		}
	}
	return false
}

// GetByProTxHash returns an index of the validator with protxhash and validator
// itself (copy) if found. Otherwise, -1 and nil are returned.
func (vals *ValidatorSet) GetByProTxHash(proTxHash []byte) (index int32, val *Validator) {
	for idx, val := range vals.Validators {
		if bytes.Equal(val.ProTxHash, proTxHash) {
			return int32(idx), val.Copy()
		}
	}
	return -1, nil
}

// GetByAddress returns an index of the validator with address and validator
// itself (copy) if found. Otherwise, -1 and nil are returned.
func (vals *ValidatorSet) GetByAddress(address []byte) (index int32, val *Validator) {
	for idx, val := range vals.Validators {
		if bytes.Equal(val.Address, address) {
			return int32(idx), val.Copy()
		}
	}
	return -1, nil
}

// GetByIndex returns the validator's address and validator itself (copy) by
// index.
// It returns nil values if index is less than 0 or greater or equal to
// len(ValidatorSet.Validators).
func (vals *ValidatorSet) GetByIndex(index int32) (proTxHash crypto.ProTxHash, val *Validator) {
	if index < 0 || int(index) >= len(vals.Validators) {
		return nil, nil
	}
	val = vals.Validators[index]
	return val.ProTxHash, val.Copy()
}

// GetProTxHashes returns the all validator proTxHashes
func (vals *ValidatorSet) GetProTxHashes() []crypto.ProTxHash {
	proTxHashes := make([]crypto.ProTxHash, len(vals.Validators))
	for i, val := range vals.Validators {
		proTxHashes[i] = val.ProTxHash
	}
	return proTxHashes
}

// GetProTxHashes returns the all validator proTxHashes as byte arrays for convenience
func (vals *ValidatorSet) GetProTxHashesAsByteArrays() [][]byte {
	proTxHashes := make([][]byte, len(vals.Validators))
	for i, val := range vals.Validators {
		proTxHashes[i] = val.ProTxHash
	}
	return proTxHashes
}

// GetPublicKeys returns the all validator publicKeys
func (vals *ValidatorSet) GetPublicKeys() []crypto.PubKey {
	publicKeys := make([]crypto.PubKey, len(vals.Validators))
	for i, val := range vals.Validators {
		publicKeys[i] = val.PubKey
	}
	return publicKeys
}

// GetAddresses returns the all validator publicKeys
func (vals *ValidatorSet) GetAddresses() []crypto.Address {
	addresses := make([]crypto.Address, len(vals.Validators))
	for i, val := range vals.Validators {
		if val.Address != nil {
			addresses[i] = val.Address
		} else {
			addresses[i] = val.PubKey.Address()
		}
	}
	return addresses
}

func (vals *ValidatorSet) GetAddressPowers() map[string]int64 {
	addresses := make(map[string]int64, len(vals.Validators))
	for _, val := range vals.Validators {
		if val.Address != nil {
			addresses[string(val.Address)] = val.VotingPower
		} else {
			addresses[string(val.PubKey.Address())] = val.VotingPower
		}
	}
	return addresses
}

// GetProTxHashes returns the all validator proTxHashes
func (vals *ValidatorSet) GetProTxHashesOrdered() []crypto.ProTxHash {
	proTxHashes := make([]crypto.ProTxHash, len(vals.Validators))
	for i, val := range vals.Validators {
		proTxHashes[i] = val.ProTxHash
	}
	sort.Sort(crypto.SortProTxHash(proTxHashes))
	return proTxHashes
}

// Size returns the length of the validator set.
func (vals *ValidatorSet) Size() int {
	return len(vals.Validators)
}

func (vals *ValidatorSet) RegenerateWithNewKeys() (*ValidatorSet, []PrivValidator) {
	var (
		proTxHashes    = vals.GetProTxHashes()
		numValidators  = len(vals.Validators)
		valz           = make([]*Validator, numValidators)
		privValidators = make([]PrivValidator, numValidators)
	)
	privateKeys, thresholdPublicKey := bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(proTxHashes)

	for i := 0; i < numValidators; i++ {
		privValidators[i] = NewMockPVWithParams(privateKeys[i], proTxHashes[i], false, false)
		valz[i] = NewValidatorDefaultVotingPower(privateKeys[i].PubKey(), proTxHashes[i])
	}

	// Just to make sure
	sort.Sort(PrivValidatorsByProTxHash(privValidators))

	return NewValidatorSet(valz, thresholdPublicKey), privValidators
}

// Forces recalculation of the set's total voting power.
// Panics if total voting power is bigger than MaxTotalVotingPower.
func (vals *ValidatorSet) updateTotalVotingPower() {
	sum := int64(0)
	for _, val := range vals.Validators {
		// mind overflow
		sum = safeAddClip(sum, val.VotingPower)
		if sum > MaxTotalVotingPower {
			panic(fmt.Sprintf(
				"Total voting power should be guarded to not exceed %v; got: %v",
				MaxTotalVotingPower,
				sum))
		}
	}

	vals.totalVotingPower = sum
}

// TotalVotingPower returns the sum of the voting powers of all validators.
// It recomputes the total voting power if required.
func (vals *ValidatorSet) TotalVotingPower() int64 {
	if vals.totalVotingPower == 0 {
		vals.updateTotalVotingPower()
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
		if proposer == nil || !bytes.Equal(val.ProTxHash, proposer.ProTxHash) {
			proposer = proposer.CompareProposerPriority(val)
		}
	}
	return proposer
}

// Hash returns the Merkle root hash build using validators (as leaves) in the
// set.
func (vals *ValidatorSet) Hash() []byte {
	bzs := make([][]byte, len(vals.Validators))
	for i, val := range vals.Validators {
		bzs[i] = val.Bytes()
	}
	return merkle.HashFromByteSlices(bzs)
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

// Checks changes against duplicates, splits the changes in updates and
// removals, sorts them by address.
//
// Returns:
// updates, removals - the sorted lists of updates and removals
// err - non-nil if duplicate entries or entries with negative voting power are seen
//
// No changes are made to 'origChanges'.
func processChanges(origChanges []*Validator) (updates, removals []*Validator, err error) {
	// Make a deep copy of the changes and sort by proTxHash.
	changes := validatorListCopy(origChanges)
	sort.Sort(ValidatorsByProTxHashes(changes))

	removals = make([]*Validator, 0, len(changes))
	updates = make([]*Validator, 0, len(changes))
	var prevProTxHash []byte

	// Scan changes by proTxHash and append valid validators to updates or removals lists.
	for _, valUpdate := range changes {
		if bytes.Equal(valUpdate.ProTxHash, prevProTxHash) {
			err = fmt.Errorf("duplicate entry %v in %v", valUpdate, changes)
			return nil, nil, err
		}

		switch {
		case valUpdate.VotingPower < 0:
			err = fmt.Errorf("voting power can't be negative: %d", valUpdate.VotingPower)
			return nil, nil, err
		case valUpdate.VotingPower > MaxTotalVotingPower:
			err = fmt.Errorf("to prevent clipping/overflow, voting power can't be higher than %d, got %d",
				MaxTotalVotingPower, valUpdate.VotingPower)
			return nil, nil, err
		case valUpdate.VotingPower == 0:
			removals = append(removals, valUpdate)
		default:
			updates = append(updates, valUpdate)
		}

		prevProTxHash = valUpdate.ProTxHash
	}

	return updates, removals, err
}

// verifyUpdates verifies a list of updates against a validator set, making sure the allowed
// total voting power would not be exceeded if these updates would be applied to the set.
//
// Inputs:
// updates - a list of proper validator changes, i.e. they have been verified by processChanges for duplicates
//   and invalid values.
// vals - the original validator set. Note that vals is NOT modified by this function.
// removedPower - the total voting power that will be removed after the updates are verified and applied.
//
// Returns:
// tvpAfterUpdatesBeforeRemovals -  the new total voting power if these updates would be applied without the removals.
//   Note that this will be < 2 * MaxTotalVotingPower in case high power validators are removed and
//   validators are added/ updated with high power values.
//
// err - non-nil if the maximum allowed total voting power would be exceeded
func verifyUpdates(
	updates []*Validator,
	vals *ValidatorSet,
	removedPower int64,
) (tvpAfterUpdatesBeforeRemovals int64, err error) {

	delta := func(update *Validator, vals *ValidatorSet) int64 {
		_, val := vals.GetByProTxHash(update.ProTxHash)
		if val != nil {
			return update.VotingPower - val.VotingPower
		}
		return update.VotingPower
	}

	for _, val := range updates {
		if val.VotingPower != 0 && val.VotingPower != 100 {
			return 0, fmt.Errorf("voting power of a node can only be 0 or 100")
		}
	}

	updatesCopy := validatorListCopy(updates)
	sort.Slice(updatesCopy, func(i, j int) bool {
		return delta(updatesCopy[i], vals) < delta(updatesCopy[j], vals)
	})

	tvpAfterRemovals := vals.TotalVotingPower() - removedPower
	for _, upd := range updatesCopy {
		tvpAfterRemovals += delta(upd, vals)
		if tvpAfterRemovals > MaxTotalVotingPower {
			return 0, ErrTotalVotingPowerOverflow
		}
	}
	return tvpAfterRemovals + removedPower, nil
}

func numNewValidators(updates []*Validator, vals *ValidatorSet) int {
	numNewValidators := 0
	for _, valUpdate := range updates {
		if !vals.HasProTxHash(valUpdate.ProTxHash) {
			numNewValidators++
		}
	}
	return numNewValidators
}

// computeNewPriorities computes the proposer priority for the validators not present in the set based on
// 'updatedTotalVotingPower'.
// Leaves unchanged the priorities of validators that are changed.
//
// 'updates' parameter must be a list of unique validators to be added or updated.
//
// 'updatedTotalVotingPower' is the total voting power of a set where all updates would be applied but
//   not the removals. It must be < 2*MaxTotalVotingPower and may be close to this limit if close to
//   MaxTotalVotingPower will be removed. This is still safe from overflow since MaxTotalVotingPower is maxInt64/8.
//
// No changes are made to the validator set 'vals'.
func computeNewPriorities(updates []*Validator, vals *ValidatorSet, updatedTotalVotingPower int64) {
	for _, valUpdate := range updates {
		proTxHash := valUpdate.ProTxHash
		_, val := vals.GetByProTxHash(proTxHash)
		if val == nil {
			// add val
			// Set ProposerPriority to -C*totalVotingPower (with C ~= 1.125) to make sure validators can't
			// un-bond and then re-bond to reset their (potentially previously negative) ProposerPriority to zero.
			//
			// Contract: updatedVotingPower < 2 * MaxTotalVotingPower to ensure ProposerPriority does
			// not exceed the bounds of int64.
			//
			// Compute ProposerPriority = -1.125*totalVotingPower == -(updatedVotingPower + (updatedVotingPower >> 3)).
			valUpdate.ProposerPriority = -(updatedTotalVotingPower + (updatedTotalVotingPower >> 3))
		} else {
			valUpdate.ProposerPriority = val.ProposerPriority
		}
	}

}

// Merges the vals' validator list with the updates list.
// When two elements with same address are seen, the one from updates is selected.
// Expects updates to be a list of updates sorted by proTxHash with no duplicates or errors,
// must have been validated with verifyUpdates() and priorities computed with computeNewPriorities().
func (vals *ValidatorSet) applyUpdates(updates []*Validator) {
	existing := vals.Validators
	sort.Sort(ValidatorsByProTxHashes(existing))

	merged := make([]*Validator, len(existing)+len(updates))
	i := 0

	for len(existing) > 0 && len(updates) > 0 {
		if bytes.Compare(existing[0].ProTxHash, updates[0].ProTxHash) < 0 { // unchanged validator
			merged[i] = existing[0]
			existing = existing[1:]
		} else {
			// Apply add or update.
			merged[i] = updates[0]
			if bytes.Equal(existing[0].ProTxHash, updates[0].ProTxHash) {
				// Validator is present in both, advance existing.
				existing = existing[1:]
			}
			updates = updates[1:]
		}
		i++
	}

	// Add the elements which are left.
	for j := 0; j < len(existing); j++ {
		merged[i] = existing[j]
		i++
	}
	// OR add updates which are left.
	for j := 0; j < len(updates); j++ {
		merged[i] = updates[j]
		i++
	}

	vals.Validators = merged[:i]
}

// Checks that the validators to be removed are part of the validator set.
// No changes are made to the validator set 'vals'.
func verifyRemovals(deletes []*Validator, vals *ValidatorSet) (votingPower int64, err error) {
	removedVotingPower := int64(0)
	for _, valUpdate := range deletes {
		proTxHash := valUpdate.ProTxHash
		_, val := vals.GetByProTxHash(proTxHash)
		if val == nil {
			return removedVotingPower, fmt.Errorf("failed to find validator %X to remove", proTxHash)
		}
		removedVotingPower += val.VotingPower
	}
	if len(deletes) > len(vals.Validators) {
		panic("more deletes than validators")
	}
	return removedVotingPower, nil
}

// Removes the validators specified in 'deletes' from validator set 'vals'.
// Should not fail as verification has been done before.
// Expects vals to be sorted by address (done by applyUpdates).
func (vals *ValidatorSet) applyRemovals(deletes []*Validator) {
	existing := vals.Validators

	merged := make([]*Validator, len(existing)-len(deletes))
	i := 0

	// Loop over deletes until we removed all of them.
	for len(deletes) > 0 {
		if bytes.Equal(existing[0].ProTxHash, deletes[0].ProTxHash) {
			deletes = deletes[1:]
		} else { // Leave it in the resulting slice.
			merged[i] = existing[0]
			i++
		}
		existing = existing[1:]
	}

	// Add the elements which are left.
	for j := 0; j < len(existing); j++ {
		merged[i] = existing[j]
		i++
	}

	vals.Validators = merged[:i]
}

// Main function used by UpdateWithChangeSet() and NewValidatorSet().
// If 'allowDeletes' is false then delete operations (identified by validators with voting power 0)
// are not allowed and will trigger an error if present in 'changes'.
// The 'allowDeletes' flag is set to false by NewValidatorSet() and to true by UpdateWithChangeSet().
func (vals *ValidatorSet) updateWithChangeSet(changes []*Validator, allowDeletes bool,
		newThresholdPublicKey crypto.PubKey) error {
	if len(changes) == 0 {
		return nil
	}

	if newThresholdPublicKey == nil {
		return errors.New("the threshold public key can not be nil")
	}

	// Check for duplicates within changes, split in 'updates' and 'deletes' lists (sorted).
	updates, deletes, err := processChanges(changes)
	if err != nil {
		return err
	}

	if !allowDeletes && len(deletes) != 0 {
		return fmt.Errorf("cannot process validators with voting power 0: %v", deletes)
	}

	// Check that the resulting set will not be empty.
	if numNewValidators(updates, vals) == 0 && len(vals.Validators) == len(deletes) {
		return errors.New("applying the validator changes would result in empty set")
	}

	// Verify that applying the 'deletes' against 'vals' will not result in error.
	// Get the voting power that is going to be removed.
	removedVotingPower, err := verifyRemovals(deletes, vals)
	if err != nil {
		return err
	}

	// Verify that applying the 'updates' against 'vals' will not result in error.
	// Get the updated total voting power before removal. Note that this is < 2 * MaxTotalVotingPower
	tvpAfterUpdatesBeforeRemovals, err := verifyUpdates(updates, vals, removedVotingPower)
	if err != nil {
		return err
	}

	// Compute the priorities for updates.
	computeNewPriorities(updates, vals, tvpAfterUpdatesBeforeRemovals)

	// Apply updates and removals.
	vals.applyUpdates(updates)
	vals.applyRemovals(deletes)

	vals.updateTotalVotingPower() // will panic if total voting power > MaxTotalVotingPower

	// Scale and center.
	vals.RescalePriorities(PriorityWindowSizeFactor * vals.TotalVotingPower())
	vals.shiftByAvgProposerPriority()

	sort.Sort(ValidatorsByVotingPower(vals.Validators))

	vals.ThresholdPublicKey = newThresholdPublicKey

	return nil
}

// UpdateWithChangeSet attempts to update the validator set with 'changes'.
// It performs the following steps:
// - validates the changes making sure there are no duplicates and splits them in updates and deletes
// - verifies that applying the changes will not result in errors
// - computes the total voting power BEFORE removals to ensure that in the next steps the priorities
//   across old and newly added validators are fair
// - computes the priorities of new validators against the final set
// - applies the updates against the validator set
// - applies the removals against the validator set
// - performs scaling and centering of priority values
// If an error is detected during verification steps, it is returned and the validator set
// is not changed.
func (vals *ValidatorSet) UpdateWithChangeSet(changes []*Validator, newThresholdPublicKey crypto.PubKey) error {
	return vals.updateWithChangeSet(changes, true, newThresholdPublicKey)
}

// VerifyCommit verifies +2/3 of the set had signed the given commit.
//
// It checks all the signatures! While it's safe to exit as soon as we have
// 2/3+ signatures, doing so would impact incentivization logic in the ABCI
// application that depends on the LastCommitInfo sent in BeginBlock, which
// includes which validators signed. For instance, Gaia incentivizes proposers
// with a bonus for including more than +2/3 of the signatures.
func (vals *ValidatorSet) VerifyCommit(chainID string, blockID BlockID, stateID StateID,
	height int64, commit *Commit) error {

	if vals.Size() != len(commit.Signatures) {
		return NewErrInvalidCommitSignatures(vals.Size(), len(commit.Signatures))
	}

	// Validate Height and BlockID.
	if height != commit.Height {
		return NewErrInvalidCommitHeight(height, commit.Height)
	}
	if !blockID.Equals(commit.BlockID) {
		return fmt.Errorf("invalid commit -- wrong block ID: want %v, got %v",
			blockID, commit.BlockID)
	}

	if !stateID.Equals(commit.StateID) {
		return fmt.Errorf("invalid commit -- wrong state ID: want %v, got %v",
			stateID, commit.StateID)
	}

	talliedVotingPower := int64(0)

	votingPowerNeedMoreThan := vals.TotalVotingPower() * 2 / 3
	for idx, commitSig := range commit.Signatures {
		if commitSig.Absent() {
			continue // OK, some signatures can be absent.
		}

		// The vals and commit have a 1-to-1 correspondence.
		// This means we don't need the validator address or to do any lookup.
		val := vals.Validators[idx]

		// Validate block signature.
		voteBlockSignBytes := commit.VoteBlockSignBytes(chainID, int32(idx))
		if !val.PubKey.VerifySignature(voteBlockSignBytes, commitSig.BlockSignature) {
			return fmt.Errorf("wrong block signature (#%d/proTxHash:%X/pubKey:%X) | voteBlockSignBytes : %X |" +
				" signature : %X | commitBID: %s | vote :%v | commit sig %v", idx, val.ProTxHash, val.PubKey.Bytes(),
				voteBlockSignBytes, commitSig.BlockSignature, commit.BlockID.String(), commit.GetVote(int32(idx)),
				commit.Signatures[idx])
		}
		// else {
		//	fmt.Printf("correct block signature  (#%d/proTxHash:%X/pubKey:%X) | voteBlockSignBytes : %X |" +
		//  	" signature : %X | commitBID: %s | vote :%v | commit sig %v\n", idx, val.ProTxHash, val.PubKey.Bytes(),
		// 	voteBlockSignBytes, commitSig.BlockSignature, commit.BlockID.String(), commit.GetVote(int32(idx)),
		//	commit.Signatures[idx])
		// }

		// Validate state signature.
		if commitSig.BlockIDFlag == BlockIDFlagCommit {
			// Only verify signatures that voted to commit the block
			voteStateSignBytes := commit.VoteStateSignBytes(chainID, int32(idx))
			if !val.PubKey.VerifySignature(voteStateSignBytes, commitSig.StateSignature) {
				return fmt.Errorf("wrong state signature (#%d/proTxHash:%X/pubKey:%X) |" +
					" voteStateSignBytes : %X | signature : %X", idx, val.ProTxHash, val.PubKey.Bytes(),
					voteStateSignBytes, commitSig.StateSignature)
			}
		}

		// Good!
		if commitSig.ForBlock() {
			talliedVotingPower += val.VotingPower
		}
		// else {
		// It's OK. We include stray signatures (~votes for nil) to measure
		// validator availability.
		// }
	}

	canonicalVoteBlockSignBytes := commit.CanonicalVoteVerifySignBytes(chainID)
	if !vals.ThresholdPublicKey.VerifySignature(canonicalVoteBlockSignBytes, commit.ThresholdBlockSignature) {
		return fmt.Errorf("incorrect threshold block signature %X %X", canonicalVoteBlockSignBytes,
			commit.ThresholdBlockSignature)
	}

	canonicalVoteStateSignBytes := commit.CanonicalVoteStateSignBytes(chainID)
	if !vals.ThresholdPublicKey.VerifySignature(canonicalVoteStateSignBytes, commit.ThresholdStateSignature) {
		return fmt.Errorf("incorrect threshold state signature %X %X", canonicalVoteStateSignBytes,
			commit.ThresholdStateSignature)
	}

	if got, needed := talliedVotingPower, votingPowerNeedMoreThan; got <= needed {
		return ErrNotEnoughVotingPowerSigned{Got: got, Needed: needed}
	}

	return nil
}

// LIGHT CLIENT VERIFICATION METHODS

// VerifyCommitLight verifies +2/3 of the set had signed the given commit.
//
// This method is primarily used by the light client and does not check all the
// signatures.
func (vals *ValidatorSet) VerifyCommitLight(chainID string, blockID BlockID, stateID StateID,
	height int64, commit *Commit) error {

	if vals.Size() != len(commit.Signatures) {
		return NewErrInvalidCommitSignatures(vals.Size(), len(commit.Signatures))
	}

	// Validate Height and BlockID.
	if height != commit.Height {
		return NewErrInvalidCommitHeight(height, commit.Height)
	}
	if !blockID.Equals(commit.BlockID) {
		return fmt.Errorf("invalid commit -- wrong block ID: want %v, got %v",
			blockID, commit.BlockID)
	}

	if !stateID.Equals(commit.StateID) {
		return fmt.Errorf("invalid commit -- wrong state ID: want %v, got %v",
			stateID, commit.StateID)
	}

	talliedVotingPower := int64(0)

	votingPowerNeeded := vals.TotalVotingPower() * 2 / 3
	for idx, commitSig := range commit.Signatures {
		// No need to verify absent or nil votes.
		if !commitSig.ForBlock() {
			continue
		}

		// The vals and commit have a 1-to-1 correspondance.
		// This means we don't need the validator address or to do any lookup.
		val := vals.Validators[idx]

		// Validate block signature.
		voteBlockSignBytes := commit.VoteBlockSignBytes(chainID, int32(idx))
		if !val.PubKey.VerifySignature(voteBlockSignBytes, commitSig.BlockSignature) {
			return fmt.Errorf("wrong block signature for light (#%d/proTxHash:%X/pubKey:%X) |" +
				" voteBlockSignBytes : %X | signature : %X | commitBID: %s | vote :%v | commit sig %v", idx,
				val.ProTxHash, val.PubKey.Bytes(), voteBlockSignBytes, commitSig.BlockSignature,
				commit.BlockID.String(), commit.GetVote(int32(idx)), commit.Signatures[idx])
		}

		// Validate block signature.
		voteStateSignBytes := commit.VoteStateSignBytes(chainID, int32(idx))
		if !val.PubKey.VerifySignature(voteStateSignBytes, commitSig.StateSignature) {
			return fmt.Errorf("wrong state signature (#%d): %X", idx, commitSig.StateSignature)
		}

		talliedVotingPower += val.VotingPower

		// return as soon as +2/3 of the signatures are verified
		if talliedVotingPower > votingPowerNeeded {
			canonicalVoteBlockSignBytes := commit.CanonicalVoteVerifySignBytes(chainID)
			if !vals.ThresholdPublicKey.VerifySignature(canonicalVoteBlockSignBytes, commit.ThresholdBlockSignature) {
				return fmt.Errorf("incorrect threshold block signature %X %X", canonicalVoteBlockSignBytes,
					commit.ThresholdBlockSignature)
			}

			canonicalVoteStateSignBytes := commit.CanonicalVoteStateSignBytes(chainID)
			if !vals.ThresholdPublicKey.VerifySignature(canonicalVoteStateSignBytes, commit.ThresholdStateSignature) {
				return fmt.Errorf("incorrect threshold state signature %X %X", canonicalVoteStateSignBytes,
					commit.ThresholdStateSignature)
			}

			return nil
		}
	}

	return ErrNotEnoughVotingPowerSigned{Got: talliedVotingPower, Needed: votingPowerNeeded}
}

// VerifyCommitLightTrusting verifies that trustLevel of the validator set signed
// this commit.
//
// NOTE the given validators do not necessarily correspond to the validator set
// for this commit, but there may be some intersection.
//
// This method is primarily used by the light client and does not check all the
// signatures.
func (vals *ValidatorSet) VerifyCommitLightTrusting(chainID string, commit *Commit, trustLevel tmmath.Fraction) error {
	// sanity check
	if trustLevel.Denominator == 0 {
		return errors.New("trustLevel has zero Denominator")
	}

	var (
		talliedVotingPower int64
		seenVals           = make(map[int32]int, len(commit.Signatures)) // validator index -> commit index
	)

	// Safely calculate voting power needed.
	totalVotingPowerMulByNumerator, overflow := safeMul(vals.TotalVotingPower(), int64(trustLevel.Numerator))
	if overflow {
		return errors.New("int64 overflow while calculating voting power needed. please provide" +
			" smaller trustLevel numerator")
	}
	votingPowerNeeded := totalVotingPowerMulByNumerator / int64(trustLevel.Denominator)

	for idx, commitSig := range commit.Signatures {
		// No need to verify absent or nil votes.
		if !commitSig.ForBlock() {
			continue
		}

		// We don't know the validators that committed this block, so we have to
		// check for each vote if its validator is already known.
		valIdx, val := vals.GetByProTxHash(commitSig.ValidatorProTxHash)

		if val != nil {
			// check for double vote of validator on the same commit
			if firstIndex, ok := seenVals[valIdx]; ok {
				secondIndex := idx
				return fmt.Errorf("double vote from %v (%d and %d)", val, firstIndex, secondIndex)
			}
			seenVals[valIdx] = idx

			// Validate block signature.
			voteBlockSignBytes := commit.VoteBlockSignBytes(chainID, int32(idx))
			if !val.PubKey.VerifySignature(voteBlockSignBytes, commitSig.BlockSignature) {
				return fmt.Errorf("wrong block signature for light trusting (#%d/proTxHash:%X/pubKey:%X) |" +
					" voteBlockSignBytes : %X | signature : %X | commitBID: %s | vote :%v | commit sig %v", idx,
					val.ProTxHash, val.PubKey.Bytes(), voteBlockSignBytes, commitSig.BlockSignature,
					commit.BlockID.String(), commit.GetVote(int32(idx)), commit.Signatures[idx])
			}

			// Validate block signature.
			voteStateSignBytes := commit.VoteStateSignBytes(chainID, int32(idx))
			if !val.PubKey.VerifySignature(voteStateSignBytes, commitSig.StateSignature) {
				return fmt.Errorf("wrong state signature for light trusting (#%d): %X", idx,
					commitSig.StateSignature)
			}

			talliedVotingPower += val.VotingPower

			if talliedVotingPower > votingPowerNeeded {
				canonicalVoteBlockSignBytes := commit.CanonicalVoteVerifySignBytes(chainID)
				if !vals.ThresholdPublicKey.VerifySignature(canonicalVoteBlockSignBytes, commit.ThresholdBlockSignature) {
					return fmt.Errorf("incorrect threshold block signature %X %X", canonicalVoteBlockSignBytes,
						commit.ThresholdBlockSignature)
				}

				canonicalVoteStateSignBytes := commit.CanonicalVoteStateSignBytes(chainID)
				if !vals.ThresholdPublicKey.VerifySignature(canonicalVoteStateSignBytes, commit.ThresholdStateSignature) {
					return fmt.Errorf("incorrect threshold state signature %X %X", canonicalVoteStateSignBytes,
						commit.ThresholdStateSignature)
				}
				return nil
			}
		}
	}

	return ErrNotEnoughVotingPowerSigned{Got: talliedVotingPower, Needed: votingPowerNeeded}
}

// findPreviousProposer reverses the compare proposer priority function to find the validator
// with the lowest proposer priority which would have been the previous proposer.
//
// Is used when recreating a validator set from an existing array of validators.
func (vals *ValidatorSet) findPreviousProposer() *Validator {
	var previousProposer *Validator
	for _, val := range vals.Validators {
		if previousProposer == nil {
			previousProposer = val
			continue
		}
		if previousProposer == previousProposer.CompareProposerPriority(val) {
			previousProposer = val
		}
	}
	return previousProposer
}

//-----------------

// IsErrNotEnoughVotingPowerSigned returns true if err is
// ErrNotEnoughVotingPowerSigned.
func IsErrNotEnoughVotingPowerSigned(err error) bool {
	return errors.As(err, &ErrNotEnoughVotingPowerSigned{})
}

// ErrNotEnoughVotingPowerSigned is returned when not enough validators signed
// a commit.
type ErrNotEnoughVotingPowerSigned struct {
	Got    int64
	Needed int64
}

func (e ErrNotEnoughVotingPowerSigned) Error() string {
	return fmt.Sprintf("invalid commit -- insufficient voting power: got %d, needed more than %d", e.Got, e.Needed)
}

func (vals *ValidatorSet) ABCIEquivalentValidatorUpdates() *abci.ValidatorSetUpdate {
	var valUpdates []abci.ValidatorUpdate
	for i := 0; i < len(vals.Validators); i++ {
		valUpdate := TM2PB.NewValidatorUpdate(vals.Validators[i].PubKey, DefaultDashVotingPower,
			vals.Validators[i].ProTxHash)
		valUpdates = append(valUpdates, valUpdate)
	}
	abciThresholdPublicKey, err := cryptoenc.PubKeyToProto(vals.ThresholdPublicKey)
	if err != nil {
		panic(err)
	}
	return &abci.ValidatorSetUpdate{
		ValidatorUpdates:   valUpdates,
		ThresholdPublicKey: abciThresholdPublicKey,
	}
}

//----------------

// String returns a string representation of ValidatorSet.
//
// See StringIndented.
func (vals *ValidatorSet) String() string {
	return vals.StringIndented("")
}

// StringIndented returns an intended String.
//
// See Validator#String.
func (vals *ValidatorSet) StringIndented(indent string) string {
	if vals == nil {
		return "nil-ValidatorSet"
	}
	var valStrings []string
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

// ValidatorsByVotingPower implements sort.Interface for []*Validator based on
// the VotingPower and Address fields.
type ValidatorsByVotingPower []*Validator

func (valz ValidatorsByVotingPower) Len() int { return len(valz) }

func (valz ValidatorsByVotingPower) Less(i, j int) bool {
	if valz[i].VotingPower == valz[j].VotingPower {
		return bytes.Compare(valz[i].ProTxHash, valz[j].ProTxHash) == -1
	}
	return valz[i].VotingPower > valz[j].VotingPower
}

func (valz ValidatorsByVotingPower) Swap(i, j int) {
	valz[i], valz[j] = valz[j], valz[i]
}

// ValidatorsByAddress implements sort.Interface for []*Validator based on
// the Address field.
type ValidatorsByProTxHashes []*Validator

func (valz ValidatorsByProTxHashes) Len() int { return len(valz) }

func (valz ValidatorsByProTxHashes) Less(i, j int) bool {
	return bytes.Compare(valz[i].ProTxHash, valz[j].ProTxHash) == -1
}

func (valz ValidatorsByProTxHashes) Swap(i, j int) {
	valz[i], valz[j] = valz[j], valz[i]
}

// ToProto converts ValidatorSet to protobuf
func (vals *ValidatorSet) ToProto() (*tmproto.ValidatorSet, error) {
	if vals.IsNilOrEmpty() {
		return &tmproto.ValidatorSet{}, nil // validator set should never be nil
	}

	vp := new(tmproto.ValidatorSet)
	valsProto := make([]*tmproto.Validator, len(vals.Validators))
	for i := 0; i < len(vals.Validators); i++ {
		valp, err := vals.Validators[i].ToProto()
		if err != nil {
			return nil, err
		}
		valsProto[i] = valp
	}
	vp.Validators = valsProto

	valProposer, err := vals.Proposer.ToProto()
	if err != nil {
		return nil, fmt.Errorf("toProto: validatorSet proposer error: %w", err)
	}
	vp.Proposer = valProposer

	vp.TotalVotingPower = vals.totalVotingPower

	if vals.ThresholdPublicKey == nil {
		return nil, fmt.Errorf("thresholdPublicKey is not set")
	}

	thresholdPublicKey, err := cryptoenc.PubKeyToProto(vals.ThresholdPublicKey)
	if err != nil {
		return nil, fmt.Errorf("toProto: thresholdPublicKey error: %w", err)
	}
	vp.ThresholdPublicKey = thresholdPublicKey

	return vp, nil
}

// ValidatorSetFromProto sets a protobuf ValidatorSet to the given pointer.
// It returns an error if any of the validators from the set or the proposer
// is invalid
func ValidatorSetFromProto(vp *tmproto.ValidatorSet) (*ValidatorSet, error) {
	if vp == nil {
		return nil, errors.New("nil validator set") // validator set should never be nil
		// bigger issues are at play if empty
	}
	vals := new(ValidatorSet)

	valsProto := make([]*Validator, len(vp.Validators))
	for i := 0; i < len(vp.Validators); i++ {
		v, err := ValidatorFromProto(vp.Validators[i])
		if err != nil {
			return nil, fmt.Errorf("fromProto: validatorSet validator error: %w", err)
		}
		valsProto[i] = v
	}
	vals.Validators = valsProto

	p, err := ValidatorFromProto(vp.GetProposer())
	if err != nil {
		return nil, fmt.Errorf("fromProto: validatorSet proposer error: %w", err)
	}

	vals.Proposer = p

	vals.totalVotingPower = vp.GetTotalVotingPower()

	thresholdPublicKey, err := cryptoenc.PubKeyFromProto(vp.ThresholdPublicKey)
	if err != nil {
		return nil, fmt.Errorf("fromProto: thresholdPublicKey error: %w", err)
	}

	vals.ThresholdPublicKey = thresholdPublicKey

	return vals, vals.ValidateBasic()
}

// ValidatorSetFromExistingValidators takes an existing array of validators and rebuilds
// the exact same validator set that corresponds to it without changing the proposer priority or power
// if any of the validators fail validate basic then an empty set is returned.
func ValidatorSetFromExistingValidators(valz []*Validator, thresholdPublicKey crypto.PubKey) (*ValidatorSet, error) {
	for _, val := range valz {
		err := val.ValidateBasic()
		if err != nil {
			return nil, fmt.Errorf("can't create validator set: %w", err)
		}
	}
	vals := &ValidatorSet{
		Validators:         valz,
		ThresholdPublicKey: thresholdPublicKey,
	}
	vals.Proposer = vals.findPreviousProposer()
	vals.updateTotalVotingPower()
	sort.Sort(ValidatorsByVotingPower(vals.Validators))
	return vals, nil
}

//----------------------------------------

// GenerateValidatorSet returns a randomized validator set (size: +numValidators+),
// where each validator has the same default voting power.
//
// EXPOSED FOR TESTING.
func GenerateValidatorSet(numValidators int) (*ValidatorSet, []PrivValidator) {
	var (
		valz           = make([]*Validator, numValidators)
		privValidators = make([]PrivValidator, numValidators)
	)
	threshold := numValidators*2/3 + 1
	privateKeys, proTxHashes, thresholdPublicKey := bls12381.CreatePrivLLMQData(numValidators, threshold)

	for i := 0; i < numValidators; i++ {
		privValidators[i] = NewMockPVWithParams(privateKeys[i], proTxHashes[i], false, false)
		valz[i] = NewValidatorDefaultVotingPower(privateKeys[i].PubKey(), proTxHashes[i])
	}

	sort.Sort(PrivValidatorsByProTxHash(privValidators))

	return NewValidatorSet(valz, thresholdPublicKey), privValidators
}

func GenerateTestValidatorSetWithAddresses(addresses []crypto.Address, power []int64) (*ValidatorSet, []PrivValidator) {
	var (
		numValidators      = len(addresses)
		proTxHashes        = make([]ProTxHash, numValidators)
		originalAddressMap = make(map[string][]byte)
		originalPowerMap   = make(map[string]int64)
		valz               = make([]*Validator, numValidators)
		privValidators     = make([]PrivValidator, numValidators)
	)
	for i := 0; i < numValidators; i++ {
		proTxHashes[i] = crypto.Sha256(addresses[i])
		originalAddressMap[string(proTxHashes[i])] = addresses[i]
		originalPowerMap[string(proTxHashes[i])] = power[i]
	}
	sort.Sort(crypto.SortProTxHash(proTxHashes))

	privateKeys, thresholdPublicKey := bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(proTxHashes)

	for i := 0; i < numValidators; i++ {
		privValidators[i] = NewMockPVWithParams(privateKeys[i], proTxHashes[i], false,
			false)
		valz[i] = NewValidator(privateKeys[i].PubKey(), originalPowerMap[string(proTxHashes[i])], proTxHashes[i])
		valz[i].Address = originalAddressMap[string(proTxHashes[i])]
	}

	sort.Sort(PrivValidatorsByProTxHash(privValidators))

	return NewValidatorSet(valz, thresholdPublicKey), privValidators
}

func GenerateTestValidatorSetWithAddressesDefaultPower(addresses []crypto.Address) (*ValidatorSet, []PrivValidator) {
	var (
		numValidators      = len(addresses)
		proTxHashes        = make([]ProTxHash, numValidators)
		originalAddressMap = make(map[string][]byte)
		valz               = make([]*Validator, numValidators)
		privValidators     = make([]PrivValidator, numValidators)
	)
	for i := 0; i < numValidators; i++ {
		proTxHashes[i] = crypto.Sha256(addresses[i])
		originalAddressMap[string(proTxHashes[i])] = addresses[i]
	}
	sort.Sort(crypto.SortProTxHash(proTxHashes))

	privateKeys, thresholdPublicKey := bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(proTxHashes)

	for i := 0; i < numValidators; i++ {
		privValidators[i] = NewMockPVWithParams(privateKeys[i], proTxHashes[i], false,
			false)
		valz[i] = NewValidatorDefaultVotingPower(privateKeys[i].PubKey(), proTxHashes[i])
		valz[i].Address = originalAddressMap[string(proTxHashes[i])]
	}

	sort.Sort(PrivValidatorsByProTxHash(privValidators))

	return NewValidatorSet(valz, thresholdPublicKey), privValidators
}

func GenerateMockValidatorSet(numValidators int) (*ValidatorSet, []*MockPV) {
	var (
		valz           = make([]*Validator, numValidators)
		privValidators = make([]*MockPV, numValidators)
	)
	threshold := numValidators*2/3 + 1
	privateKeys, proTxHashes, thresholdPublicKey := bls12381.CreatePrivLLMQData(numValidators, threshold)

	for i := 0; i < numValidators; i++ {
		privValidators[i] = NewMockPVWithParams(privateKeys[i], proTxHashes[i], false,
			false)
		valz[i] = NewValidatorDefaultVotingPower(privateKeys[i].PubKey(), proTxHashes[i])
	}

	sort.Sort(MockPrivValidatorsByProTxHash(privValidators))

	return NewValidatorSet(valz, thresholdPublicKey), privValidators
}

func GenerateGenesisValidators(numValidators int) ([]GenesisValidator, []PrivValidator, crypto.PubKey) {
	var (
		genesisValidators = make([]GenesisValidator, numValidators)
		privValidators    = make([]PrivValidator, numValidators)
	)
	privateKeys, proTxHashes, thresholdPublicKey := bls12381.CreatePrivLLMQDataDefaultThreshold(numValidators)

	for i := 0; i < numValidators; i++ {
		privValidators[i] = NewMockPVWithParams(privateKeys[i], proTxHashes[i], false,
			false)
		genesisValidators[i] = GenesisValidator{
			PubKey:    privateKeys[i].PubKey(),
			Power:     DefaultDashVotingPower,
			ProTxHash: proTxHashes[i],
		}
	}

	sort.Sort(PrivValidatorsByProTxHash(privValidators))
	sort.Sort(GenesisValidatorsByProTxHash(genesisValidators))

	return genesisValidators, privValidators, thresholdPublicKey
}

func GenerateMockGenesisValidators(numValidators int) ([]GenesisValidator, []*MockPV, crypto.PubKey) {
	var (
		genesisValidators = make([]GenesisValidator, numValidators)
		privValidators    = make([]*MockPV, numValidators)
	)
	privateKeys, proTxHashes, thresholdPublicKey := bls12381.CreatePrivLLMQDataDefaultThreshold(numValidators)

	for i := 0; i < numValidators; i++ {
		privValidators[i] = NewMockPVWithParams(privateKeys[i], proTxHashes[i], false,
			false)
		genesisValidators[i] = GenesisValidator{
			PubKey:    privateKeys[i].PubKey(),
			Power:     DefaultDashVotingPower,
			ProTxHash: proTxHashes[i],
		}
	}

	sort.Sort(MockPrivValidatorsByProTxHash(privValidators))
	sort.Sort(GenesisValidatorsByProTxHash(genesisValidators))

	return genesisValidators, privValidators, thresholdPublicKey
}

func GenerateValidatorSetUsingProTxHashes(proTxHashes []crypto.ProTxHash) (*ValidatorSet, []PrivValidator) {
	numValidators := len(proTxHashes)
	if numValidators < 2 {
		panic("there should be at least 2 validators")
	}
	var (
		valz           = make([]*Validator, numValidators)
		privValidators = make([]PrivValidator, numValidators)
	)
	privateKeys, thresholdPublicKey := bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(proTxHashes)

	for i := 0; i < numValidators; i++ {
		privValidators[i] = NewMockPVWithParams(privateKeys[i], proTxHashes[i], false,
			false)
		valz[i] = NewValidatorDefaultVotingPower(privateKeys[i].PubKey(), proTxHashes[i])
	}

	sort.Sort(PrivValidatorsByProTxHash(privValidators))

	return NewValidatorSet(valz, thresholdPublicKey), privValidators
}

func GenerateMockValidatorSetUsingProTxHashes(proTxHashes []crypto.ProTxHash) (*ValidatorSet, []*MockPV) {
	numValidators := len(proTxHashes)
	if numValidators < 2 {
		panic("there should be at least 2 validators")
	}
	var (
		valz           = make([]*Validator, numValidators)
		privValidators = make([]*MockPV, numValidators)
	)
	privateKeys, thresholdPublicKey := bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(proTxHashes)

	for i := 0; i < numValidators; i++ {
		privValidators[i] = NewMockPVWithParams(privateKeys[i], proTxHashes[i], false,
			false)
		valz[i] = NewValidatorDefaultVotingPower(privateKeys[i].PubKey(), proTxHashes[i])
	}

	sort.Sort(MockPrivValidatorsByProTxHash(privValidators))

	return NewValidatorSet(valz, thresholdPublicKey), privValidators
}

func ValidatorUpdatesRegenerateOnProTxHashes(proTxHashes []crypto.ProTxHash) abci.ValidatorSetUpdate {
	privateKeys, thresholdPublicKey := bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(proTxHashes)
	var valUpdates []abci.ValidatorUpdate
	for i := 0; i < len(proTxHashes); i++ {
		valUpdate := TM2PB.NewValidatorUpdate(privateKeys[i].PubKey(), DefaultDashVotingPower, proTxHashes[i])
		valUpdates = append(valUpdates, valUpdate)
	}
	abciThresholdPublicKey, err := cryptoenc.PubKeyToProto(thresholdPublicKey)
	if err != nil {
		panic(err)
	}
	return abci.ValidatorSetUpdate{
		ValidatorUpdates:   valUpdates,
		ThresholdPublicKey: abciThresholdPublicKey,
	}
}

// safe addition/subtraction/multiplication

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

func safeMul(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, false
	}

	absOfB := b
	if b < 0 {
		absOfB = -b
	}

	absOfA := a
	if a < 0 {
		absOfA = -a
	}

	if absOfA > math.MaxInt64/absOfB {
		return 0, true
	}

	return a * b, false
}
