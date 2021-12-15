package factory

import (
	"context"
	"fmt"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"math/rand"
	"sort"

	"github.com/tendermint/tendermint/types"
)

func GenerateTestValidatorSetWithProTxHashes(
	proTxHashes []crypto.ProTxHash,
	power []int64,
) (*types.ValidatorSet, []types.PrivValidator) {
	var (
		numValidators    = len(proTxHashes)
		originalPowerMap = make(map[string]int64)
		valz             = make([]*types.Validator, numValidators)
		privValidators   = make([]types.PrivValidator, numValidators)
	)
	for i := 0; i < numValidators; i++ {
		originalPowerMap[string(proTxHashes[i])] = power[i]
	}

	sortedProTxHashes := proTxHashes
	sort.Sort(crypto.SortProTxHash(sortedProTxHashes))

	orderedProTxHashes, privateKeys, thresholdPublicKey :=
		bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(sortedProTxHashes)
	quorumHash := crypto.RandQuorumHash()

	for i := 0; i < numValidators; i++ {
		privValidators[i] = types.NewMockPVWithParams(privateKeys[i], orderedProTxHashes[i], quorumHash,
			thresholdPublicKey, false, false)
		valz[i] = types.NewValidator(
			privateKeys[i].PubKey(),
			originalPowerMap[string(orderedProTxHashes[i])],
			orderedProTxHashes[i],
		)
	}

	sort.Sort(types.PrivValidatorsByProTxHash(privValidators))

	return types.NewValidatorSet(valz, thresholdPublicKey, crypto.SmallQuorumType(), quorumHash, true), privValidators
}

func GenerateTestValidatorSetWithProTxHashesDefaultPower(
	proTxHashes []crypto.ProTxHash,
) (*types.ValidatorSet, []types.PrivValidator) {
	var (
		numValidators  = len(proTxHashes)
		valz           = make([]*types.Validator, numValidators)
		privValidators = make([]types.PrivValidator, numValidators)
	)
	sort.Sort(crypto.SortProTxHash(proTxHashes))

	orderedProTxHashes, privateKeys, thresholdPublicKey :=
		bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(proTxHashes)
	quorumHash := crypto.RandQuorumHash()

	for i := 0; i < numValidators; i++ {
		privValidators[i] = types.NewMockPVWithParams(privateKeys[i], orderedProTxHashes[i], quorumHash,
			thresholdPublicKey, false, false)
		valz[i] = types.NewValidatorDefaultVotingPower(privateKeys[i].PubKey(), orderedProTxHashes[i])
	}

	sort.Sort(types.PrivValidatorsByProTxHash(privValidators))

	return types.NewValidatorSet(valz, thresholdPublicKey, crypto.SmallQuorumType(), quorumHash, true), privValidators
}

func GenerateMockValidatorSet(numValidators int) (*types.ValidatorSet, []*types.MockPV) {
	var (
		valz           = make([]*types.Validator, numValidators)
		privValidators = make([]*types.MockPV, numValidators)
	)
	threshold := numValidators*2/3 + 1
	privateKeys, proTxHashes, thresholdPublicKey := bls12381.CreatePrivLLMQData(numValidators, threshold)
	quorumHash := crypto.RandQuorumHash()

	for i := 0; i < numValidators; i++ {
		privValidators[i] = types.NewMockPVWithParams(
			privateKeys[i],
			proTxHashes[i],
			quorumHash,
			thresholdPublicKey,
			false,
			false,
		)
		valz[i] = types.NewValidatorDefaultVotingPower(privateKeys[i].PubKey(), proTxHashes[i])
	}

	sort.Sort(types.MockPrivValidatorsByProTxHash(privValidators))

	return types.NewValidatorSet(valz, thresholdPublicKey, crypto.SmallQuorumType(), quorumHash, true), privValidators
}

func GenerateGenesisValidators(
	numValidators int,
) ([]types.GenesisValidator, []types.PrivValidator, crypto.QuorumHash, crypto.PubKey) {
	var (
		genesisValidators = make([]types.GenesisValidator, numValidators)
		privValidators    = make([]types.PrivValidator, numValidators)
	)
	privateKeys, proTxHashes, thresholdPublicKey :=
		bls12381.CreatePrivLLMQDataDefaultThreshold(numValidators)
	quorumHash := crypto.RandQuorumHash()

	for i := 0; i < numValidators; i++ {
		privValidators[i] = types.NewMockPVWithParams(privateKeys[i], proTxHashes[i], quorumHash,
			thresholdPublicKey, false, false)
		genesisValidators[i] = types.GenesisValidator{
			PubKey:    privateKeys[i].PubKey(),
			Power:     types.DefaultDashVotingPower,
			ProTxHash: proTxHashes[i],
		}
	}

	sort.Sort(types.PrivValidatorsByProTxHash(privValidators))
	sort.Sort(types.GenesisValidatorsByProTxHash(genesisValidators))

	return genesisValidators, privValidators, quorumHash, thresholdPublicKey
}

func GenerateMockGenesisValidators(
	numValidators int,
) ([]types.GenesisValidator, []*types.MockPV, crypto.QuorumHash, crypto.PubKey) {
	var (
		genesisValidators = make([]types.GenesisValidator, numValidators)
		privValidators    = make([]*types.MockPV, numValidators)
	)
	privateKeys, proTxHashes, thresholdPublicKey :=
		bls12381.CreatePrivLLMQDataDefaultThreshold(numValidators)
	quorumHash := crypto.RandQuorumHash()

	for i := 0; i < numValidators; i++ {
		privValidators[i] = types.NewMockPVWithParams(
			privateKeys[i],
			proTxHashes[i],
			quorumHash,
			thresholdPublicKey,
			false,
			false,
		)
		genesisValidators[i] = types.GenesisValidator{
			PubKey:    privateKeys[i].PubKey(),
			Power:     types.DefaultDashVotingPower,
			ProTxHash: proTxHashes[i],
		}
	}

	sort.Sort(types.MockPrivValidatorsByProTxHash(privValidators))
	sort.Sort(types.GenesisValidatorsByProTxHash(genesisValidators))

	return genesisValidators, privValidators, quorumHash, thresholdPublicKey
}

func GenerateValidatorSetUsingProTxHashes(proTxHashes []crypto.ProTxHash) (*types.ValidatorSet, []types.PrivValidator) {
	numValidators := len(proTxHashes)
	if numValidators < 2 {
		panic("there should be at least 2 validators")
	}
	var (
		valz           = make([]*types.Validator, numValidators)
		privValidators = make([]types.PrivValidator, numValidators)
	)
	orderedProTxHashes, privateKeys, thresholdPublicKey :=
		bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(proTxHashes)
	quorumHash := crypto.RandQuorumHash()

	for i := 0; i < numValidators; i++ {
		privValidators[i] = types.NewMockPVWithParams(
			privateKeys[i],
			orderedProTxHashes[i],
			quorumHash,
			thresholdPublicKey,
			false,
			false,
		)
		valz[i] = types.NewValidatorDefaultVotingPower(privateKeys[i].PubKey(), orderedProTxHashes[i])
	}

	sort.Sort(types.PrivValidatorsByProTxHash(privValidators))

	return types.NewValidatorSet(valz, thresholdPublicKey, crypto.SmallQuorumType(), quorumHash, true), privValidators
}

func GenerateMockValidatorSetUsingProTxHashes(proTxHashes []crypto.ProTxHash) (*types.ValidatorSet, []*types.MockPV) {
	numValidators := len(proTxHashes)
	if numValidators < 2 {
		panic("there should be at least 2 validators")
	}
	var (
		valz           = make([]*types.Validator, numValidators)
		privValidators = make([]*types.MockPV, numValidators)
	)
	orderedProTxHashes, privateKeys, thresholdPublicKey :=
		bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(proTxHashes)
	quorumHash := crypto.RandQuorumHash()

	for i := 0; i < numValidators; i++ {
		privValidators[i] = types.NewMockPVWithParams(privateKeys[i], orderedProTxHashes[i], quorumHash,
			thresholdPublicKey, false, false)
		valz[i] = types.NewValidatorDefaultVotingPower(privateKeys[i].PubKey(), orderedProTxHashes[i])
	}

	sort.Sort(types.MockPrivValidatorsByProTxHash(privValidators))

	return types.NewValidatorSet(valz, thresholdPublicKey, crypto.SmallQuorumType(), quorumHash, true), privValidators
}

func GenerateMockValidatorSetUpdatingPrivateValidatorsAtHeight(
	proTxHashes []crypto.ProTxHash,
	mockPVs map[string]*types.MockPV,
	height int64,
) (*types.ValidatorSet, []*types.MockPV) {
	numValidators := len(mockPVs)
	if numValidators < 2 {
		panic("there should be at least 2 validators")
	}
	var (
		valz           = make([]*types.Validator, numValidators)
		privValidators = make([]*types.MockPV, numValidators)
	)
	orderedProTxHashes, privateKeys, thresholdPublicKey :=
		bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(proTxHashes)
	quorumHash := crypto.RandQuorumHash()

	for i := 0; i < numValidators; i++ {
		if mockPV, ok := mockPVs[orderedProTxHashes[i].String()]; ok {
			mockPV.UpdatePrivateKey(context.Background(), privateKeys[i], quorumHash, thresholdPublicKey, height)
			privValidators[i] = mockPV
		} else {
			privValidators[i] = types.NewMockPVWithParams(privateKeys[i], orderedProTxHashes[i], quorumHash,
				thresholdPublicKey, false, false)
		}
		valz[i] = types.NewValidatorDefaultVotingPower(privateKeys[i].PubKey(), orderedProTxHashes[i])
	}

	sort.Sort(types.MockPrivValidatorsByProTxHash(privValidators))

	return types.NewValidatorSet(valz, thresholdPublicKey, crypto.SmallQuorumType(), quorumHash, true), privValidators
}


func RandValidator() (*types.Validator, types.PrivValidator) {
	quorumHash := crypto.RandQuorumHash()
	privVal := types.NewMockPVForQuorum(quorumHash)
	proTxHash, err := privVal.GetProTxHash(context.Background())
	if err != nil {
		panic(fmt.Errorf("could not retrieve proTxHash %w", err))
	}
	pubKey, err := privVal.GetPubKey(context.Background(), quorumHash)
	if err != nil {
		panic(fmt.Errorf("could not retrieve pubkey %w", err))
	}
	val := types.NewValidatorDefaultVotingPower(pubKey, proTxHash)
	return val, privVal
}

// RandValidatorSet returns a randomized validator set (size: +numValidators+),
// where each validator has the same default voting power.
//
func RandValidatorSet(numValidators int) (*types.ValidatorSet, []types.PrivValidator) {
	return RandValidatorSetWithPriority(numValidators, false)
}

// RandValidatorSetWithPriority returns a randomized validator set (size: +numValidators+),
// where each validator has the same default voting power, if shouldGeneratePriority is set a random priority is set
// to each validator
//
func RandValidatorSetWithPriority(numValidators int, shouldGeneratePriority bool) (*types.ValidatorSet, []types.PrivValidator) {
	var (
		valz           = make([]*types.Validator, numValidators)
		privValidators = make([]types.PrivValidator, numValidators)
	)
	threshold := numValidators*2/3 + 1
	privateKeys, proTxHashes, thresholdPublicKey :=
		bls12381.CreatePrivLLMQData(numValidators, threshold)
	quorumHash := crypto.RandQuorumHash()

	for i := 0; i < numValidators; i++ {
		privValidators[i] = types.NewMockPVWithParams(privateKeys[i], proTxHashes[i], quorumHash,
			thresholdPublicKey, false, false)
		valz[i] = types.NewValidatorDefaultVotingPower(privateKeys[i].PubKey(), proTxHashes[i])
		if shouldGeneratePriority {
			valz[i].ProposerPriority = rand.Int63() % (types.MaxTotalVotingPower - (int64(numValidators)*types.DefaultDashVotingPower))
		}
	}

	sort.Sort(types.PrivValidatorsByProTxHash(privValidators))

	return types.NewValidatorSet(valz, thresholdPublicKey, crypto.SmallQuorumType(), quorumHash, true), privValidators
}
