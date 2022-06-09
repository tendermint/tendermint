package factory

import (
	"sort"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/dash/llmq"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/types"
)

func RandGenesisDoc(
	cfg *config.Config,
	numValidators int,
	initialHeight int64,
	consensusParams *types.ConsensusParams,
) (*types.GenesisDoc, []types.PrivValidator) {
	validators := make([]types.GenesisValidator, 0, numValidators)
	privValidators := make([]types.PrivValidator, 0, numValidators)

	ld := llmq.MustGenerate(crypto.RandProTxHashes(numValidators))
	quorumHash := crypto.RandQuorumHash()
	iter := ld.Iter()
	for iter.Next() {
		proTxHash, qks := iter.Value()
		validators = append(validators, types.GenesisValidator{
			PubKey:    qks.PubKey,
			Power:     types.DefaultDashVotingPower,
			ProTxHash: proTxHash,
		})
		privValidators = append(privValidators, types.NewMockPVWithParams(
			qks.PrivKey,
			proTxHash,
			quorumHash,
			ld.ThresholdPubKey,
			false,
			false,
		))
	}
	sort.Sort(types.PrivValidatorsByProTxHash(privValidators))

	coreChainLock := types.NewMockChainLock(2)

	return &types.GenesisDoc{
		GenesisTime:     tmtime.Now(),
		InitialHeight:   initialHeight,
		ChainID:         cfg.ChainID(),
		Validators:      validators,
		ConsensusParams: consensusParams,

		// dash fields
		InitialCoreChainLockedHeight: 1,
		InitialProposalCoreChainLock: coreChainLock.ToProto(),
		ThresholdPublicKey:           ld.ThresholdPubKey,
		QuorumHash:                   quorumHash,
	}, privValidators
}
