package state

import (
	"io"

	. "github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/merkle"
)

// Not goroutine-safe.
type ValidatorSet struct {
	validators       merkle.Tree
	proposer         *Validator // Whoever has the highest Accum.
	totalVotingPower uint64
}

func NewValidatorSet(vals []*Validator) *ValidatorSet {
	validators := merkle.NewIAVLTree(BasicCodec, ValidatorCodec, 0, nil) // In memory
	var proposer *Validator
	totalVotingPower := uint64(0)
	for _, val := range vals {
		validators.Set(val.Id, val)
		proposer = proposer.CompareAccum(val)
		totalVotingPower += val.VotingPower
	}
	return &ValidatorSet{
		validators:       validators,
		proposer:         proposer,
		totalVotingPower: totalVotingPower,
	}
}

func ReadValidatorSet(r io.Reader, n *int64, err *error) *ValidatorSet {
	size := ReadUInt64(r, n, err)
	validators := []*Validator{}
	for i := uint64(0); i < size; i++ {
		validator := ReadValidator(r, n, err)
		validators = append(validators, validator)
	}
	return NewValidatorSet(validators)
}

func (vset *ValidatorSet) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt64(w, uint64(vset.validators.Size()), &n, &err)
	vset.validators.Iterate(func(key_ interface{}, val_ interface{}) bool {
		val := val_.(*Validator)
		WriteBinary(w, val, &n, &err)
		return false
	})
	return
}

func (vset *ValidatorSet) IncrementAccum() {
	// Decrement from previous proposer
	vset.proposer.Accum -= int64(vset.totalVotingPower)
	var proposer *Validator
	// Increment accum and find proposer
	vset.validators.Iterate(func(key_ interface{}, val_ interface{}) bool {
		val := val_.(*Validator)
		val.Accum += int64(val.VotingPower)
		proposer = proposer.CompareAccum(val)
		return false
	})
	vset.proposer = proposer
}

func (vset *ValidatorSet) Copy() *ValidatorSet {
	return &ValidatorSet{
		validators:       vset.validators.Copy(),
		proposer:         vset.proposer,
		totalVotingPower: vset.totalVotingPower,
	}
}

func (vset *ValidatorSet) GetById(id uint64) (index uint32, val *Validator) {
	index_, val_ := vset.validators.Get(id)
	index, val = uint32(index_), val_.(*Validator)
	return
}

func (vset *ValidatorSet) GetByIndex(index uint32) (id uint64, val *Validator) {
	id_, val_ := vset.validators.GetByIndex(uint64(index))
	id, val = id_.(uint64), val_.(*Validator)
	return
}

func (vset *ValidatorSet) Size() uint {
	return uint(vset.validators.Size())
}

func (vset *ValidatorSet) TotalVotingPower() uint64 {
	return vset.totalVotingPower
}

func (vset *ValidatorSet) Proposer() (proposer *Validator) {
	return vset.proposer
}

func (vset *ValidatorSet) Hash() []byte {
	return vset.validators.Hash()
}

func (vset *ValidatorSet) Add(val *Validator) (added bool) {
	if vset.validators.Has(val.Id) {
		return false
	}
	return !vset.validators.Set(val.Id, val)
}

func (vset *ValidatorSet) Update(val *Validator) (updated bool) {
	if !vset.validators.Has(val.Id) {
		return false
	}
	return vset.validators.Set(val.Id, val)
}

func (vset *ValidatorSet) Remove(validatorId uint64) (val *Validator, removed bool) {
	val_, removed := vset.validators.Remove(validatorId)
	return val_.(*Validator), removed
}

func (vset *ValidatorSet) Iterate(fn func(val *Validator) bool) {
	vset.validators.Iterate(func(key_ interface{}, val_ interface{}) bool {
		return fn(val_.(*Validator))
	})
}
