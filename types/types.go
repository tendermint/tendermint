package types

import (
	"fmt"
	"github.com/tendermint/go-crypto"
)

//---------------------------------------------
// simple types

// Known chain and validator set IDs (from which anything else can be found)
type ChainAndValidatorIDs struct {
	ChainIDs        []string `json:"chain_ids"`
	ValidatorSetIDs []string `json:"validator_set_ids"`
}

// basic chain status/metrics
type BlockchainStatus struct {
	Height        int     `json:"height"`
	MeanBlockTime float64 `json:"mean_block_time" wire:"unsafe"`
	TxThroughput  float64 `json:"tx_throughput" wire:"unsafe"`

	BlockchainSize int64 `json:"blockchain_size"` // how might we get StateSize ?
}

// validator set (independent of chains)
type ValidatorSet struct {
	Validators []*Validator `json:"validators"`
}

func (vs *ValidatorSet) Validator(valID string) (*Validator, error) {
	for _, v := range vs.Validators {
		if v.ID == valID {
			return v, nil
		}
	}
	return nil, fmt.Errorf("Unknwon validator %s", valID)
}

// validator (independent of chain)
type Validator struct {
	ID     string        `json:"id"`
	PubKey crypto.PubKey `json:"pub_key"`
	Chains []string      `json:"chains"`
}
