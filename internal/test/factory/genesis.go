package factory

import (
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"sort"

	"github.com/tendermint/tendermint/config"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/types"
)

func RandGenesisDoc(cfg *config.Config, numValidators int, initialHeight int64) (*types.GenesisDoc, []types.PrivValidator) {
	validators := make([]types.GenesisValidator, numValidators)
	privValidators := make([]types.PrivValidator, numValidators)

	privateKeys, proTxHashes, thresholdPublicKey := bls12381.CreatePrivLLMQDataDefaultThreshold(numValidators)
	quorumHash := crypto.RandQuorumHash()
	for i := 0; i < numValidators; i++ {
		val := types.NewValidatorDefaultVotingPower(privateKeys[i].PubKey(), proTxHashes[i])
		validators[i] = types.GenesisValidator{
			PubKey:    val.PubKey,
			Power:     val.VotingPower,
			ProTxHash: val.ProTxHash,
		}
		privValidators[i] = types.NewMockPVWithParams(privateKeys[i], proTxHashes[i], quorumHash, thresholdPublicKey,
			false, false)
	}
	sort.Sort(types.PrivValidatorsByProTxHash(privValidators))

	coreChainLock := types.NewMockChainLock(2)

	return &types.GenesisDoc{
		GenesisTime:                  tmtime.Now(),
		InitialHeight:                initialHeight,
		ChainID:                      cfg.ChainID(),
		Validators:                   validators,
		InitialCoreChainLockedHeight: 1,
		InitialProposalCoreChainLock: coreChainLock.ToProto(),
		ThresholdPublicKey:           thresholdPublicKey,
		QuorumHash:                   quorumHash,
	}, privValidators
}
