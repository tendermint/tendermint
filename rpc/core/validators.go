package core

import (
	sm "github.com/tendermint/tendermint/state"
)

//-----------------------------------------------------------------------------

func ListValidators() (uint, []*sm.Validator, []*sm.Validator) {
	var blockHeight uint
	var bondedValidators []*sm.Validator
	var unbondingValidators []*sm.Validator

	state := consensusState.GetState()
	blockHeight = state.LastBlockHeight
	state.BondedValidators.Iterate(func(index uint, val *sm.Validator) bool {
		bondedValidators = append(bondedValidators, val)
		return false
	})
	state.UnbondingValidators.Iterate(func(index uint, val *sm.Validator) bool {
		unbondingValidators = append(unbondingValidators, val)
		return false
	})

	return blockHeight, bondedValidators, unbondingValidators
}
