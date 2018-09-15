package state

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// database keys
var (
	stateKey = []byte("stateKey")
)

//-----------------------------------------------------------------------------

// State is a short description of the latest committed block of the Tendermint consensus.
// It keeps all information necessary to validate new blocks,
// including the last validator set and the consensus params.
// All fields are exposed so the struct can be easily serialized,
// but none of them should be mutated directly.
// Instead, use state.Copy() or state.NextState(...).
// NOTE: not goroutine-safe.
type State struct {
	// immutable
	ChainID string

	// LastBlockHeight=0 at genesis (ie. block(H=0) does not exist)
	LastBlockHeight  int64
	LastBlockTotalTx int64
	LastBlockID      types.BlockID
	LastBlockTime    time.Time

	// LastValidators is used to validate block.LastCommit.
	// Validators are persisted to the database separately every time they change,
	// so we can query for historical validator sets.
	// Note that if s.LastBlockHeight causes a valset change,
	// we set s.LastHeightValidatorsChanged = s.LastBlockHeight + 1
	NextValidators              *types.ValidatorSet
	Validators                  *types.ValidatorSet
	LastValidators              *types.ValidatorSet
	LastHeightValidatorsChanged int64

	// Consensus parameters used for validating blocks.
	// Changes returned by EndBlock and updated after Commit.
	ConsensusParams                  types.ConsensusParams
	LastHeightConsensusParamsChanged int64

	// Merkle root of the results from executing prev block
	LastResultsHash []byte

	// the latest AppHash we've received from calling abci.Commit()
	AppHash []byte
}

// Copy makes a copy of the State for mutating.
func (state State) Copy() State {
	return State{
		ChainID: state.ChainID,

		LastBlockHeight:  state.LastBlockHeight,
		LastBlockTotalTx: state.LastBlockTotalTx,
		LastBlockID:      state.LastBlockID,
		LastBlockTime:    state.LastBlockTime,

		NextValidators:              state.NextValidators.Copy(),
		Validators:                  state.Validators.Copy(),
		LastValidators:              state.LastValidators.Copy(),
		LastHeightValidatorsChanged: state.LastHeightValidatorsChanged,

		ConsensusParams:                  state.ConsensusParams,
		LastHeightConsensusParamsChanged: state.LastHeightConsensusParamsChanged,

		AppHash: state.AppHash,

		LastResultsHash: state.LastResultsHash,
	}
}

// Equals returns true if the States are identical.
func (state State) Equals(state2 State) bool {
	sbz, s2bz := state.Bytes(), state2.Bytes()
	return bytes.Equal(sbz, s2bz)
}

// Bytes serializes the State using go-amino.
func (state State) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(state)
}

// IsEmpty returns true if the State is equal to the empty State.
func (state State) IsEmpty() bool {
	return state.Validators == nil // XXX can't compare to Empty
}

//------------------------------------------------------------------------
// Create a block from the latest state

// MakeBlock builds a block from the current state with the given txs, commit,
// and evidence. Note it also takes a proposerAddress because the state does not
// track rounds, and hence doesn't know the correct proposer. TODO: alleviate this!
func (state State) MakeBlock(
	height int64,
	txs []types.Tx,
	commit *types.Commit,
	evidence []types.Evidence,
	proposerAddress []byte,
) (*types.Block, *types.PartSet) {

	// Build base block with block data.
	block := types.MakeBlock(height, txs, commit, evidence)

	// Fill rest of header with state data.
	block.ChainID = state.ChainID

	// Set time
	if height == 1 {
		block.Time = tmtime.Now()
		if block.Time.Before(state.LastBlockTime) {
			block.Time = state.LastBlockTime // state.LastBlockTime for height == 1 is genesis time
		}
	} else {
		block.Time = MedianTime(commit, state.LastValidators)
	}

	block.LastBlockID = state.LastBlockID
	block.TotalTxs = state.LastBlockTotalTx + block.NumTxs

	block.ValidatorsHash = state.Validators.Hash()
	block.NextValidatorsHash = state.NextValidators.Hash()
	block.ConsensusHash = state.ConsensusParams.Hash()
	block.AppHash = state.AppHash
	block.LastResultsHash = state.LastResultsHash

	// NOTE: we can't use the state.Validators because we don't
	// IncrementAccum for rounds there.
	block.ProposerAddress = proposerAddress

	return block, block.MakePartSet(types.BlockPartSizeBytes)
}

// MedianTime computes a median time for a given Commit (based on Timestamp field of votes messages) and the
// corresponding validator set. The computed time is always between timestamps of
// the votes sent by honest processes, i.e., a faulty processes can not arbitrarily increase or decrease the
// computed value.
func MedianTime(commit *types.Commit, validators *types.ValidatorSet) time.Time {

	weightedTimes := make([]*tmtime.WeightedTime, len(commit.Precommits))
	totalVotingPower := int64(0)

	for i, vote := range commit.Precommits {
		if vote != nil {
			_, validator := validators.GetByIndex(vote.ValidatorIndex)
			totalVotingPower += validator.VotingPower
			weightedTimes[i] = tmtime.NewWeightedTime(vote.Timestamp, validator.VotingPower)
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
func MakeGenesisStateFromFile(genDocFile string) (State, error) {
	genDoc, err := MakeGenesisDocFromFile(genDocFile)
	if err != nil {
		return State{}, err
	}
	return MakeGenesisState(genDoc)
}

// MakeGenesisDocFromFile reads and unmarshals genesis doc from the given file.
func MakeGenesisDocFromFile(genDocFile string) (*types.GenesisDoc, error) {
	genDocJSON, err := ioutil.ReadFile(genDocFile)
	if err != nil {
		return nil, fmt.Errorf("Couldn't read GenesisDoc file: %v", err)
	}
	genDoc, err := types.GenesisDocFromJSON(genDocJSON)
	if err != nil {
		return nil, fmt.Errorf("Error reading GenesisDoc: %v", err)
	}
	return genDoc, nil
}

// MakeGenesisState creates state from types.GenesisDoc.
func MakeGenesisState(genDoc *types.GenesisDoc) (State, error) {
	err := genDoc.ValidateAndComplete()
	if err != nil {
		return State{}, fmt.Errorf("Error in genesis file: %v", err)
	}

	// Make validators slice
	validators := make([]*types.Validator, len(genDoc.Validators))
	for i, val := range genDoc.Validators {
		pubKey := val.PubKey
		address := pubKey.Address()

		// Make validator
		validators[i] = &types.Validator{
			Address:     address,
			PubKey:      pubKey,
			VotingPower: val.Power,
		}
	}

	return State{

		ChainID: genDoc.ChainID,

		LastBlockHeight: 0,
		LastBlockID:     types.BlockID{},
		LastBlockTime:   genDoc.GenesisTime,

		NextValidators:              types.NewValidatorSet(validators).CopyIncrementAccum(1),
		Validators:                  types.NewValidatorSet(validators),
		LastValidators:              types.NewValidatorSet(nil),
		LastHeightValidatorsChanged: 1,

		ConsensusParams:                  *genDoc.ConsensusParams,
		LastHeightConsensusParamsChanged: 1,

		AppHash: genDoc.AppHash,
	}, nil
}
