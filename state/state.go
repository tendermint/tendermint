package state

import (
	"fmt"
	"io/ioutil"
	"time"

	tmstate "github.com/tendermint/tendermint/proto/state"
	tmproto "github.com/tendermint/tendermint/proto/types"
	protoversion "github.com/tendermint/tendermint/proto/version"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
	"github.com/tendermint/tendermint/version"
)

// database keys
var (
	stateKey = []byte("stateKey")
)

//-----------------------------------------------------------------------------

// initStateVersion sets the Consensus.Block and Software versions,
// but leaves the Consensus.App version blank.
// The Consensus.App version will be set during the Handshake, once
// we hear from the app what protocol version it is running.
var initStateVersion = tmstate.Version{
	Consensus: protoversion.Consensus{
		Block: version.BlockProtocol,
		App:   0,
	},
	Software: version.TMCoreSemVer,
}

//-----------------------------------------------------------------------------

// Copy makes a copy of the State for mutating.
func CopyState(state tmstate.State) tmstate.State {

	return tmstate.State{
		Version: state.Version,
		ChainID: state.ChainID,

		LastBlockHeight: state.LastBlockHeight,
		LastBlockID:     state.LastBlockID,
		LastBlockTime:   state.LastBlockTime,

		NextValidators:              types.CopyProtoValSet(state.NextValidators),
		Validators:                  types.CopyProtoValSet(state.Validators),
		LastValidators:              types.CopyProtoValSet(state.LastValidators),
		LastHeightValidatorsChanged: state.LastHeightValidatorsChanged,

		ConsensusParams:                  state.ConsensusParams,
		LastHeightConsensusParamsChanged: state.LastHeightConsensusParamsChanged,

		AppHash: state.AppHash,

		LastResultsHash: state.LastResultsHash,
	}
}

//------------------------------------------------------------------------
// Create a block from the latest state

// MakeBlock builds a block from the current state with the given txs, commit,
// and evidence. Note it also takes a proposerAddress because the state does not
// track rounds, and hence does not know the correct proposer. TODO: fix this!
func MakeBlock(
	state tmstate.State,
	height int64,
	txs []types.Tx,
	commit *types.Commit,
	evidence []types.Evidence,
	proposerAddress []byte,
) (*types.Block, *types.PartSet) {

	// Build base block with block data.
	block := types.MakeBlock(height, txs, commit, evidence)

	// Set time.
	var timestamp time.Time
	if height == 1 {
		timestamp = state.LastBlockTime // genesis time
	} else {
		var lastVals types.ValidatorSet
		if err := lastVals.FromProto(state.LastValidators); err != nil {
			return nil, nil
		}
		timestamp = MedianTime(commit, &lastVals)
	}
	var lastBi types.BlockID
	if err := lastBi.FromProto(&state.LastBlockID); err != nil {
		return nil, nil
	}

	var vals types.ValidatorSet
	if err := vals.FromProto(state.Validators); err != nil {
		return nil, nil
	}

	var nextVals types.ValidatorSet
	if err := nextVals.FromProto(state.NextValidators); err != nil {
		return nil, nil
	}

	// Fill rest of header with state data.
	block.Header.Populate(
		state.Version.Consensus, state.ChainID,
		timestamp, lastBi,
		vals.Hash(), nextVals.Hash(),
		types.HashConsensusParams(state.ConsensusParams), state.AppHash, state.LastResultsHash,
		proposerAddress,
	)

	return block, block.MakePartSet(types.BlockPartSizeBytes)
}

// MedianTime computes a median time for a given Commit (based on Timestamp field of votes messages) and the
// corresponding validator set. The computed time is always between timestamps of
// the votes sent by honest processes, i.e., a faulty processes can not arbitrarily increase or decrease the
// computed value.
func MedianTime(commit *types.Commit, validators *types.ValidatorSet) time.Time {
	weightedTimes := make([]*tmtime.WeightedTime, len(commit.Signatures))
	totalVotingPower := int64(0)

	for i, commitSig := range commit.Signatures {
		if commitSig.Absent() {
			continue
		}
		_, validator, ok := validators.GetByAddress(commitSig.ValidatorAddress)
		// If there's no condition, TestValidateBlockCommit panics; not needed normally.
		if ok {
			totalVotingPower += validator.VotingPower
			weightedTimes[i] = tmtime.NewWeightedTime(commitSig.Timestamp, validator.VotingPower)
		}
	}

	return tmtime.WeightedMedian(weightedTimes, totalVotingPower)
}

//------------------------------------------------------------------------
// Genesis

// MakeGenesisStateFromFile reads and unmarshals state from the given
// file.
//
// Used during replay and in tests.
func MakeGenesisStateFromFile(genDocFile string) (tmstate.State, error) {
	genDoc, err := MakeGenesisDocFromFile(genDocFile)
	if err != nil {
		return tmstate.State{}, err
	}
	return MakeGenesisState(genDoc)
}

// MakeGenesisDocFromFile reads and unmarshals genesis doc from the given file.
func MakeGenesisDocFromFile(genDocFile string) (*types.GenesisDoc, error) {
	genDocJSON, err := ioutil.ReadFile(genDocFile)
	if err != nil {
		return nil, fmt.Errorf("couldn't read GenesisDoc file: %v", err)
	}
	genDoc, err := types.GenesisDocFromJSON(genDocJSON)
	if err != nil {
		return nil, fmt.Errorf("error reading GenesisDoc: %v", err)
	}
	return genDoc, nil
}

// MakeGenesisState creates state from types.GenesisDoc.
func MakeGenesisState(genDoc *types.GenesisDoc) (tmstate.State, error) {
	err := genDoc.ValidateAndComplete()
	if err != nil {
		return tmstate.State{}, fmt.Errorf("error in genesis file: %v", err)
	}

	var validatorSet, nextValidatorSet *types.ValidatorSet
	if genDoc.Validators == nil {
		validatorSet = types.NewValidatorSet(nil)
		nextValidatorSet = types.NewValidatorSet(nil)
	} else {
		validators := make([]*types.Validator, len(genDoc.Validators))
		for i, val := range genDoc.Validators {
			validators[i] = types.NewValidator(val.PubKey, val.Power)
		}
		validatorSet = types.NewValidatorSet(validators)

		nextValidatorSet = types.NewValidatorSet(validators).CopyIncrementProposerPriority(1)
	}
	pbv, err := validatorSet.ToProto()
	if err != nil {
		return tmstate.State{}, err
	}

	pbnv, err := nextValidatorSet.ToProto()
	if err != nil {
		return tmstate.State{}, err
	}

	protoParam := tmproto.ConsensusParams{
		Block:     genDoc.ConsensusParams.Block,
		Evidence:  genDoc.ConsensusParams.Evidence,
		Validator: genDoc.ConsensusParams.Validator,
	}

	return tmstate.State{
		Version: initStateVersion,
		ChainID: genDoc.ChainID,

		LastBlockHeight: 0,
		LastBlockID:     tmproto.BlockID{},
		LastBlockTime:   genDoc.GenesisTime,

		NextValidators:              pbnv,
		Validators:                  pbv,
		LastValidators:              nil,
		LastHeightValidatorsChanged: 1,

		ConsensusParams:                  protoParam,
		LastHeightConsensusParamsChanged: 1,

		AppHash: genDoc.AppHash,
	}, nil
}
