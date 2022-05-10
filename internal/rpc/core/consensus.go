package core

import (
	"context"

	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/rpc/coretypes"
)

// Validators gets the validator set at the given block height.
//
// If no height is provided, it will fetch the latest validator set. Note the
// validators are sorted by their voting power - this is the canonical order
// for the validators in the set as used in computing their Merkle root.
//
// More: https://docs.tendermint.com/master/rpc/#/Info/validators
func (env *Environment) Validators(ctx context.Context, req *coretypes.RequestValidators) (*coretypes.ResultValidators, error) {
	// The latest validator that we know is the NextValidator of the last block.
	height, err := env.getHeight(env.latestUncommittedHeight(), (*int64)(req.Height))
	if err != nil {
		return nil, err
	}

	validators, err := env.StateStore.LoadValidators(height)
	if err != nil {
		return nil, err
	}

	totalCount := len(validators.Validators)
	perPage := env.validatePerPage(req.PerPage.IntPtr())
	page, err := validatePage(req.Page.IntPtr(), perPage, totalCount)
	if err != nil {
		return nil, err
	}

	skipCount := validateSkipCount(page, perPage)

	v := validators.Validators[skipCount : skipCount+tmmath.MinInt(perPage, totalCount-skipCount)]

	return &coretypes.ResultValidators{
		BlockHeight: height,
		Validators:  v,
		Count:       len(v),
		Total:       totalCount,
	}, nil
}

// DumpConsensusState dumps consensus state.
// UNSTABLE
// More: https://docs.tendermint.com/master/rpc/#/Info/dump_consensus_state
func (env *Environment) DumpConsensusState(ctx context.Context) (*coretypes.ResultDumpConsensusState, error) {
	// Get Peer consensus states.

	var peerStates []coretypes.PeerStateInfo
	peers := env.PeerManager.Peers()
	peerStates = make([]coretypes.PeerStateInfo, 0, len(peers))
	for _, pid := range peers {
		peerState, ok := env.ConsensusReactor.GetPeerState(pid)
		if !ok {
			continue
		}

		peerStateJSON, err := peerState.ToJSON()
		if err != nil {
			return nil, err
		}

		addr := env.PeerManager.Addresses(pid)
		if len(addr) != 0 {
			peerStates = append(peerStates, coretypes.PeerStateInfo{
				// Peer basic info.
				NodeAddress: addr[0].String(),
				// Peer consensus state.
				PeerState: peerStateJSON,
			})
		}
	}

	// Get self round state.
	roundState, err := env.ConsensusState.GetRoundStateJSON()
	if err != nil {
		return nil, err
	}
	return &coretypes.ResultDumpConsensusState{
		RoundState: roundState,
		Peers:      peerStates,
	}, nil
}

// ConsensusState returns a concise summary of the consensus state.
// UNSTABLE
// More: https://docs.tendermint.com/master/rpc/#/Info/consensus_state
func (env *Environment) GetConsensusState(ctx context.Context) (*coretypes.ResultConsensusState, error) {
	// Get self round state.
	bz, err := env.ConsensusState.GetRoundStateSimpleJSON()
	return &coretypes.ResultConsensusState{RoundState: bz}, err
}

// ConsensusParams gets the consensus parameters at the given block height.
// If no height is provided, it will fetch the latest consensus params.
// More: https://docs.tendermint.com/master/rpc/#/Info/consensus_params
func (env *Environment) ConsensusParams(ctx context.Context, req *coretypes.RequestConsensusParams) (*coretypes.ResultConsensusParams, error) {
	// The latest consensus params that we know is the consensus params after
	// the last block.
	height, err := env.getHeight(env.latestUncommittedHeight(), (*int64)(req.Height))
	if err != nil {
		return nil, err
	}

	consensusParams, err := env.StateStore.LoadConsensusParams(height)
	if err != nil {
		return nil, err
	}

	consensusParams.Synchrony = consensusParams.Synchrony.SynchronyParamsOrDefaults()
	consensusParams.Timeout = consensusParams.Timeout.TimeoutParamsOrDefaults()

	return &coretypes.ResultConsensusParams{
		BlockHeight:     height,
		ConsensusParams: consensusParams}, nil
}
