package state

import (
	"io"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/merkle"
)

// Holds state for a Validator at a given height+round.
// Meant to be discarded every round of the consensus protocol.
// TODO consider moving this to another common types package.
type Validator struct {
	Account
	BondHeight  uint32 // TODO: is this needed?
	VotingPower uint64
	Accum       int64
}

// Used to persist the state of ConsensusStateControl.
func ReadValidator(r io.Reader, n *int64, err *error) *Validator {
	return &Validator{
		Account:     ReadAccount(r, n, err),
		BondHeight:  ReadUInt32(r, n, err),
		VotingPower: ReadUInt64(r, n, err),
		Accum:       ReadInt64(r, n, err),
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
	WriteBinary(w, v.Account, &n, &err)
	WriteUInt32(w, v.BondHeight, &n, &err)
	WriteUInt64(w, v.VotingPower, &n, &err)
	WriteInt64(w, v.Accum, &n, &err)
	return
}

//-----------------------------------------------------------------------------

// Not goroutine-safe.
type ValidatorSet struct {
	validators       map[uint64]*Validator
	indexToId        map[uint32]uint64 // bitarray index to validator id
	idToIndex        map[uint64]uint32 // validator id to bitarray index
	totalVotingPower uint64
}

func NewValidatorSet(validators map[uint64]*Validator) *ValidatorSet {
	if validators == nil {
		validators = make(map[uint64]*Validator)
	}
	ids := []uint64{}
	indexToId := map[uint32]uint64{}
	idToIndex := map[uint64]uint32{}
	totalVotingPower := uint64(0)
	for id, val := range validators {
		ids = append(ids, id)
		totalVotingPower += val.VotingPower
	}
	UInt64Slice(ids).Sort()
	for i, id := range ids {
		indexToId[uint32(i)] = id
		idToIndex[id] = uint32(i)
	}
	return &ValidatorSet{
		validators:       validators,
		indexToId:        indexToId,
		idToIndex:        idToIndex,
		totalVotingPower: totalVotingPower,
	}
}

func (vset *ValidatorSet) IncrementAccum() {
	totalDelta := int64(0)
	for _, validator := range vset.validators {
		validator.Accum += int64(validator.VotingPower)
		totalDelta += int64(validator.VotingPower)
	}
	proposer := vset.GetProposer()
	proposer.Accum -= totalDelta
	// NOTE: sum(v) here should be zero.
	if true {
		totalAccum := int64(0)
		for _, validator := range vset.validators {
			totalAccum += validator.Accum
		}
		if totalAccum != 0 {
			Panicf("Total Accum of validators did not equal 0. Got: ", totalAccum)
		}
	}
}

func (vset *ValidatorSet) Copy() *ValidatorSet {
	validators := map[uint64]*Validator{}
	for id, val := range vset.validators {
		validators[id] = val.Copy()
	}
	return &ValidatorSet{
		validators:       validators,
		indexToId:        vset.indexToId,
		idToIndex:        vset.idToIndex,
		totalVotingPower: vset.totalVotingPower,
	}
}

func (vset *ValidatorSet) GetById(id uint64) *Validator {
	return vset.validators[id]
}

func (vset *ValidatorSet) GetIndexById(id uint64) (uint32, bool) {
	index, ok := vset.idToIndex[id]
	return index, ok
}

func (vset *ValidatorSet) GetIdByIndex(index uint32) (uint64, bool) {
	id, ok := vset.indexToId[index]
	return id, ok
}

func (vset *ValidatorSet) Map() map[uint64]*Validator {
	return vset.validators
}

func (vset *ValidatorSet) Size() uint {
	return uint(len(vset.validators))
}

func (vset *ValidatorSet) TotalVotingPower() uint64 {
	return vset.totalVotingPower
}

// TODO: cache proposer. invalidate upon increment.
func (vset *ValidatorSet) GetProposer() (proposer *Validator) {
	highestAccum := int64(0)
	for _, validator := range vset.validators {
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

// Should uniquely determine the state of the ValidatorSet.
func (vset *ValidatorSet) Hash() []byte {
	ids := []uint64{}
	for id, _ := range vset.validators {
		ids = append(ids, id)
	}
	UInt64Slice(ids).Sort()
	sortedValidators := make([]Binary, len(ids))
	for i, id := range ids {
		sortedValidators[i] = vset.validators[id]
	}
	return merkle.HashFromBinaries(sortedValidators)
}
