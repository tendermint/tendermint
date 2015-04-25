package core

import (
	"github.com/tendermint/tendermint/binary"
	cm "github.com/tendermint/tendermint/consensus"
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
	roundState := consensusState.GetRoundState()
	peerRoundStates := []string{}
	for _, peer := range p2pSwitch.Peers().List() {
		// TODO: clean this up?
		peerState := peer.Data.Get(cm.PeerStateKey).(*cm.PeerState)
		peerRoundState := peerState.GetRoundState()
		peerRoundStateStr := peer.Key + ":" + string(binary.JSONBytes(peerRoundState))
		peerRoundStates = append(peerRoundStates, peerRoundStateStr)
	}
	return &ctypes.ResponseDumpConsensusState{roundState.String(), peerRoundStates}, nil
}
