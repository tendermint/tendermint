package core

import (
	cm "github.com/eris-ltd/tendermint/consensus"
	ctypes "github.com/eris-ltd/tendermint/rpc/core/types"
	"github.com/eris-ltd/tendermint/types"
	"github.com/eris-ltd/tendermint/wire"
)

func ListValidators() (*ctypes.ResultListValidators, error) {
	var blockHeight int
	var bondedValidators []*types.Validator
	var unbondingValidators []*types.Validator

	state := consensusState.GetState()
	blockHeight = state.LastBlockHeight
	state.BondedValidators.Iterate(func(index int, val *types.Validator) bool {
		bondedValidators = append(bondedValidators, val)
		return false
	})
	state.UnbondingValidators.Iterate(func(index int, val *types.Validator) bool {
		unbondingValidators = append(unbondingValidators, val)
		return false
	})

	return &ctypes.ResultListValidators{blockHeight, bondedValidators, unbondingValidators}, nil
}

func DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	roundState := consensusState.GetRoundState()
	peerRoundStates := []string{}
	for _, peer := range p2pSwitch.Peers().List() {
		// TODO: clean this up?
		peerState := peer.Data.Get(types.PeerStateKey).(*cm.PeerState)
		peerRoundState := peerState.GetRoundState()
		peerRoundStateStr := peer.Key + ":" + string(wire.JSONBytes(peerRoundState))
		peerRoundStates = append(peerRoundStates, peerRoundStateStr)
	}
	return &ctypes.ResultDumpConsensusState{roundState.String(), peerRoundStates}, nil
}
