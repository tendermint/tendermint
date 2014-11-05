package state

import (
	"fmt"
	"io"
	"sort"
	"strings"

	. "github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/merkle"
)

//-------------------------------------
// Implements sort for sorting validators by id.

type ValidatorSlice []*Validator

func (vs ValidatorSlice) Len() int {
	return len(vs)
}

func (vs ValidatorSlice) Less(i, j int) bool {
	return vs[i].Id < vs[j].Id
}

func (vs ValidatorSlice) Swap(i, j int) {
	it := vs[i]
	vs[i] = vs[j]
	vs[j] = it
}

//-------------------------------------

// Not goroutine-safe.
// TODO: consider validator Accum overflow?
// TODO: replace validators []*Validator with github.com/jaekwon/go-ibbs?
// NOTE: all get/set to validators should copy the value for safety.
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

func ReadValidatorSet(r io.Reader, n *int64, err *error) *ValidatorSet {
	size := ReadUVarInt(r, n, err)
	validators := []*Validator{}
	for i := uint(0); i < size; i++ {
		validator := ReadValidator(r, n, err)
		validators = append(validators, validator)
	}
	sort.Sort(ValidatorSlice(validators))
	return NewValidatorSet(validators)
}

func (vset *ValidatorSet) WriteTo(w io.Writer) (n int64, err error) {
	WriteUVarInt(w, uint(len(vset.validators)), &n, &err)
	vset.Iterate(func(index uint, val *Validator) bool {
		WriteBinary(w, val, &n, &err)
		return false
	})
	return
}

func (vset *ValidatorSet) IncrementAccum() {
	// Decrement from previous proposer
	oldProposer := vset.Proposer()
	oldProposer.Accum -= int64(vset.TotalVotingPower())
	vset.Update(oldProposer)
	var newProposer *Validator
	// Increment accum and find new proposer
	// NOTE: updates validators in place.
	for _, val := range vset.validators {
		val.Accum += int64(val.VotingPower)
		newProposer = newProposer.CompareAccum(val)
	}
	vset.proposer = newProposer
}

func (vset *ValidatorSet) Copy() *ValidatorSet {
	validators := make([]*Validator, len(vset.validators))
	for i, val := range vset.validators {
		// NOTE: must copy, since IncrementAccum updates in place.
		validators[i] = val.Copy()
	}
	return &ValidatorSet{
		validators:       validators,
		proposer:         vset.proposer,
		totalVotingPower: vset.totalVotingPower,
	}
}

func (vset *ValidatorSet) HasId(id uint64) bool {
	idx := sort.Search(len(vset.validators), func(i int) bool {
		return id <= vset.validators[i].Id
	})
	return idx != len(vset.validators) && vset.validators[idx].Id == id
}

func (vset *ValidatorSet) GetById(id uint64) (index uint, val *Validator) {
	idx := sort.Search(len(vset.validators), func(i int) bool {
		return id <= vset.validators[i].Id
	})
	if idx != len(vset.validators) && vset.validators[idx].Id == id {
		return uint(idx), vset.validators[idx].Copy()
	} else {
		return 0, nil
	}
}

func (vset *ValidatorSet) GetByIndex(index uint) (id uint64, val *Validator) {
	val = vset.validators[index]
	return val.Id, val.Copy()
}

func (vset *ValidatorSet) Size() uint {
	return uint(len(vset.validators))
}

func (vset *ValidatorSet) TotalVotingPower() uint64 {
	if vset.totalVotingPower == 0 {
		for _, val := range vset.validators {
			vset.totalVotingPower += val.VotingPower
		}
	}
	return vset.totalVotingPower
}

func (vset *ValidatorSet) Proposer() (proposer *Validator) {
	if vset.proposer == nil {
		for _, val := range vset.validators {
			vset.proposer = vset.proposer.CompareAccum(val)
		}
	}
	return vset.proposer.Copy()
}

func (vset *ValidatorSet) Hash() []byte {
	if len(vset.validators) == 0 {
		return nil
	}
	hashables := make([]merkle.Hashable, len(vset.validators))
	for i, val := range vset.validators {
		hashables[i] = val
	}
	return merkle.HashFromHashables(hashables)
}

func (vset *ValidatorSet) Add(val *Validator) (added bool) {
	val = val.Copy()
	idx := sort.Search(len(vset.validators), func(i int) bool {
		return val.Id <= vset.validators[i].Id
	})
	if idx == len(vset.validators) {
		vset.validators = append(vset.validators, val)
		// Invalidate cache
		vset.proposer = nil
		vset.totalVotingPower = 0
		return true
	} else if vset.validators[idx].Id == val.Id {
		return false
	} else {
		newValidators := append(vset.validators[:idx], val)
		newValidators = append(newValidators, vset.validators[idx:]...)
		vset.validators = newValidators
		// Invalidate cache
		vset.proposer = nil
		vset.totalVotingPower = 0
		return true
	}
}

func (vset *ValidatorSet) Update(val *Validator) (updated bool) {
	index, sameVal := vset.GetById(val.Id)
	if sameVal == nil {
		return false
	} else {
		vset.validators[index] = val.Copy()
		// Invalidate cache
		vset.proposer = nil
		vset.totalVotingPower = 0
		return true
	}
}

func (vset *ValidatorSet) Remove(id uint64) (val *Validator, removed bool) {
	idx := sort.Search(len(vset.validators), func(i int) bool {
		return id <= vset.validators[i].Id
	})
	if idx == len(vset.validators) || vset.validators[idx].Id != id {
		return nil, false
	} else {
		removedVal := vset.validators[idx]
		newValidators := vset.validators[:idx]
		if idx+1 < len(vset.validators) {
			newValidators = append(newValidators, vset.validators[idx+1:]...)
		}
		vset.validators = newValidators
		// Invalidate cache
		vset.proposer = nil
		vset.totalVotingPower = 0
		return removedVal, true
	}
}

func (vset *ValidatorSet) Iterate(fn func(index uint, val *Validator) bool) {
	for i, val := range vset.validators {
		stop := fn(uint(i), val.Copy())
		if stop {
			break
		}
	}
}

func (vset *ValidatorSet) String() string {
	return vset.StringWithIndent("")
}

func (vset *ValidatorSet) StringWithIndent(indent string) string {
	valStrings := []string{}
	vset.Iterate(func(index uint, val *Validator) bool {
		valStrings = append(valStrings, val.String())
		return false
	})
	return fmt.Sprintf(`ValidatorSet{
%s  Proposer: %v
%s  Validators:
%s    %v
%s}`,
		indent, vset.Proposer().String(),
		indent,
		indent, strings.Join(valStrings, "\n"+indent+"    "),
		indent)

}
