package state

import (
	"github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
)

var config = cfg.DefaultBaseConfig()

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

func (m *ABCIResponses) UpdateDeliverTx(dtxs []*types.ResponseDeliverTx) {
	if config.DeliverBlock {
		m.DeliverBlock.DeliverTxs = dtxs
	} else {
		m.DeliverTxs = dtxs
	}
}

func (m *ABCIResponses) UpdateDeliverTxByIndex(txRes *types.ResponseDeliverTx, idx int) {
	if config.DeliverBlock {
		m.DeliverBlock.DeliverTxs[idx] = txRes
	} else {
		m.DeliverTxs[idx] = txRes
	}
}
