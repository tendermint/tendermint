package core

import (
	cm "github.com/tendermint/tendermint/consensus"
	cmn "github.com/tendermint/tendermint/libs/common"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// Validators gets the validator set at the given block height.
// If no height is provided, it will fetch the current validator set.
// Note the validators are sorted by their address - this is the canonical
// order for the validators in the set as used in computing their Merkle root.
// More: https://tendermint.com/rpc/#/Info/validators
func Validators(ctx *rpctypes.Context, heightPtr *int64, page, perPage int) (*ctypes.ResultValidators, error) {
	// The latest validator that we know is the
	// NextValidator of the last block.
	height := consensusState.GetState().LastBlockHeight + 1
	height, err := getHeight(height, heightPtr)
	if err != nil {
		return nil, err
	}

	validators, err := sm.LoadValidators(stateDB, height)
	if err != nil {
		return nil, err
	}

	totalCount := len(validators.Validators)
	perPage = validatePerPage(perPage)
	page, err = validatePage(page, perPage, totalCount)
	if err != nil {
		return nil, err
	}

	skipCount := validateSkipCount(page, perPage)

	v := validators.Validators[skipCount : skipCount+cmn.MinInt(perPage, totalCount-skipCount)]

	return &ctypes.ResultValidators{
		BlockHeight: height,
		Validators:  v}, nil
}

// DumpConsensusState dumps consensus state.
// UNSTABLE
// More: https://tendermint.com/rpc/#/Info/dump_consensus_state
func DumpConsensusState(ctx *rpctypes.Context) (*ctypes.ResultDumpConsensusState, error) {
	// Get Peer consensus states.
	peers := p2pPeers.Peers().List()
	peerStates := make([]ctypes.PeerStateInfo, len(peers))
	for i, peer := range peers {
		peerState, ok := peer.Get(types.PeerStateKey).(*cm.PeerState)
		if !ok { // peer does not have a state yet
			continue
		}
		peerStateJSON, err := peerState.ToJSON()
		if err != nil {
			return nil, err
		}
		peerStates[i] = ctypes.PeerStateInfo{
			// Peer basic info.
			NodeAddress: peer.SocketAddr().String(),
			// Peer consensus state.
			PeerState: peerStateJSON,
		}
	}
	// Get self round state.
	roundState, err := consensusState.GetRoundStateJSON()
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultDumpConsensusState{
		RoundState: roundState,
		Peers:      peerStates}, nil
}

// ConsensusState returns a concise summary of the consensus state.
// UNSTABLE
// More: https://tendermint.com/rpc/#/Info/consensus_state
func ConsensusState(ctx *rpctypes.Context) (*ctypes.ResultConsensusState, error) {
	// Get self round state.
	bz, err := consensusState.GetRoundStateSimpleJSON()
	return &ctypes.ResultConsensusState{RoundState: bz}, err
}

// ConsensusParams gets the consensus parameters  at the given block height.
// If no height is provided, it will fetch the current consensus params.
// More: https://tendermint.com/rpc/#/Info/consensus_params
func ConsensusParams(ctx *rpctypes.Context, heightPtr *int64) (*ctypes.ResultConsensusParams, error) {
	height := consensusState.GetState().LastBlockHeight + 1
	height, err := getHeight(height, heightPtr)
	if err != nil {
		return nil, err
	}

	consensusparams, err := sm.LoadConsensusParams(stateDB, height)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultConsensusParams{
		BlockHeight:     height,
		ConsensusParams: consensusparams}, nil
}
