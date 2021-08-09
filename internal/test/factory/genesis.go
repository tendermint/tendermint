package factory

import (
	"sort"

	cfg "github.com/tendermint/tendermint/config"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/pkg/consensus"
)

func RandGenesisDoc(
	config *cfg.Config,
	numValidators int,
	randPower bool,
	minPower int64) (*consensus.GenesisDoc, []consensus.PrivValidator) {

	validators := make([]consensus.GenesisValidator, numValidators)
	privValidators := make([]consensus.PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		val, privVal := RandValidator(randPower, minPower)
		validators[i] = consensus.GenesisValidator{
			PubKey: val.PubKey,
			Power:  val.VotingPower,
		}
		privValidators[i] = privVal
	}
	sort.Sort(consensus.PrivValidatorsByAddress(privValidators))

	return &consensus.GenesisDoc{
		GenesisTime:   tmtime.Now(),
		InitialHeight: 1,
		ChainID:       config.ChainID(),
		Validators:    validators,
	}, privValidators
}
