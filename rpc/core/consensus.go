package core

import (
	"github.com/tendermint/tendermint/binary"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	sm "github.com/tendermint/tendermint/state"
)

func ListValidators() (*ctypes.ResponseListValidators, error) {
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

	return &ctypes.ResponseListValidators{blockHeight, bondedValidators, unbondingValidators}, nil
}

func DumpConsensusState() (*ctypes.ResponseDumpConsensusState, error) {
	jsonBytes := binary.JSONBytes(consensusState.GetRoundState())
	return &ctypes.ResponseDumpConsensusState{string(jsonBytes)}, nil
}
