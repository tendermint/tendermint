package state

import (
	"reflect"

	"github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
)

var config = cfg.DefaultBaseConfig()

func CopyFields(dst interface{}, src interface{}) {
	sv := reflect.ValueOf(src)
	dv := reflect.ValueOf(dst)

	fields := make([]string, 0)
	for i := 0; i < sv.NumField(); i++ {
		fields = append(fields, reflect.TypeOf(src).Field(i).Name)
	}
	for _, v := range fields {
		f := dv.Elem().FieldByName(v)
		svalue := sv.FieldByName(v)
		f.Set(svalue)
	}
}

func (m *ABCIResponses) ValidatorUpdates() types.ValidatorUpdates {
	if m.EndBlock.ValidatorUpdates != nil {
		return m.EndBlock.ValidatorUpdates
	}
	var deliverBlockValidatorUpdates types.ValidatorUpdates
	for i, v := range m.DeliverBlock.ValidatorUpdates {
		CopyFields(&deliverBlockValidatorUpdates[i], v)
	}
	return deliverBlockValidatorUpdates
}

func (m *ABCIResponses) ConsensusParamUpdates() *types.ConsensusParams {
	if m.EndBlock.ConsensusParamUpdates != nil {
		return m.EndBlock.ConsensusParamUpdates
	}
	var deliverBlockConsensusParamUpdates *types.ConsensusParams
	CopyFields(&(deliverBlockConsensusParamUpdates.Evidence), m.DeliverBlock.ConsensusParamUpdates.Evidence)
	CopyFields(&(deliverBlockConsensusParamUpdates.Block), m.DeliverBlock.ConsensusParamUpdates.Block)
	CopyFields(&(deliverBlockConsensusParamUpdates.Validator), m.DeliverBlock.ConsensusParamUpdates.Validator)
	CopyFields(&(deliverBlockConsensusParamUpdates.Version), m.DeliverBlock.ConsensusParamUpdates.Version)

	return deliverBlockConsensusParamUpdates
}

func (m *ABCIResponses) UpdateDeliverTx(dtxs []*types.ResponseDeliverTx) {
	if config.DeliverBlock {
		for _, tx := range dtxs {
			CopyFields(&(m.DeliverBlock.DeliverTxs), tx)
		}
	} else {
		m.DeliverTxs = dtxs
	}
}

func (m *ABCIResponses) UpdateDeliverTxByIndex(txRes *types.ResponseDeliverTx, idx int) {
	if config.DeliverBlock {
		CopyFields(&(m.DeliverBlock.DeliverTxs[idx]), txRes)
	} else {
		m.DeliverTxs[idx] = txRes
	}
}
