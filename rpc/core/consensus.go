package core

import (
	"github.com/tendermint/go-wire"
	cm "github.com/tendermint/tendermint/consensus"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

func Validators() (*ctypes.ResultValidators, error) {
	blockHeight, validators := consensusState.GetValidators()
	return &ctypes.ResultValidators{blockHeight, validators}, nil
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
