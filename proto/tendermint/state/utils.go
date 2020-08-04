package state

import "github.com/tendermint/tendermint/abci/types"

func (m *ABCIResponses) ValidatorUpdates() types.ValidatorUpdates {
	if m.EndBlock.ValidatorUpdates != nil {
		return m.EndBlock.ValidatorUpdates
	}
	return m.DeliverBlock.ValidatorUpdates

}

func (m *ABCIResponses) ConsensusParamUpdates() *types.ConsensusParams {
	if m.EndBlock.ConsensusParamUpdates != nil {
		return m.EndBlock.ConsensusParamUpdates
	}
	return m.DeliverBlock.ConsensusParamUpdates

}
