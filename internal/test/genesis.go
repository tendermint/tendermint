package test

import (
	"time"

	cfg "github.com/tendermint/tendermint/config"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func GenesisDoc(
	config *cfg.Config,
	time time.Time,
	validators []*types.Validator,
	consensusParams *tmproto.ConsensusParams,
) *types.GenesisDoc {

	genesisValidators := make([]types.GenesisValidator, len(validators))

	for i := range validators {
		genesisValidators[i] = types.GenesisValidator{
			Power:  validators[i].VotingPower,
			PubKey: validators[i].PubKey,
		}
	}

	return &types.GenesisDoc{
		GenesisTime:     time,
		InitialHeight:   1,
		ChainID:         config.ChainID(),
		Validators:      genesisValidators,
		ConsensusParams: consensusParams,
	}
}
