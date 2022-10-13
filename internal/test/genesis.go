package test

import (
	"time"

	"github.com/tendermint/tendermint/types"
)

func ConsensusParams() *types.ConsensusParams {
	c := types.DefaultConsensusParams()
	// enable vote extensions
	c.ABCI.VoteExtensionsEnableHeight = 1
	return c
}

func GenesisDoc(
	chainID string,
	time time.Time,
	validators []*types.Validator,
	consensusParams *types.ConsensusParams,
) *types.GenesisDoc {
	if chainID == "" {
		chainID = DefaultTestChainID
	}

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
		ChainID:         chainID,
		Validators:      genesisValidators,
		ConsensusParams: consensusParams,
	}
}
