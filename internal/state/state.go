package state

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gogo/protobuf/proto"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

//-----------------------------------------------------------------------------

type Version struct {
	Consensus version.Consensus ` json:"consensus"`
	Software  string            ` json:"software"`
}

// InitStateVersion sets the Consensus.Block and Software versions,
// but leaves the Consensus.App version blank.
// The Consensus.App version will be set during the Handshake, once
// we hear from the app what protocol version it is running.
var InitStateVersion = Version{
	Consensus: version.Consensus{
		Block: version.BlockProtocol,
		App:   0,
	},
	Software: version.TMCoreSemVer,
}

func (v *Version) ToProto() tmstate.Version {
	return tmstate.Version{
		Consensus: tmversion.Consensus{
			Block: v.Consensus.Block,
			App:   v.Consensus.App,
		},
		Software: v.Software,
	}
}

func VersionFromProto(v tmstate.Version) Version {
	return Version{
		Consensus: version.Consensus{
			Block: v.Consensus.Block,
			App:   v.Consensus.App,
		},
		Software: v.Software,
	}
}

//-----------------------------------------------------------------------------

// State is a short description of the latest committed block of the Tendermint consensus.
// It keeps all information necessary to validate new blocks,
// including the last validator set and the consensus params.
// All fields are exposed so the struct can be easily serialized,
// but none of them should be mutated directly.
// Instead, use state.Copy() or updateState(...).
// NOTE: not goroutine-safe.
type State struct {
	// FIXME: This can be removed as TMVersion is a constant, and version.Consensus should
	// eventually be replaced by VersionParams in ConsensusParams
	Version Version

	// immutable
	ChainID       string
	InitialHeight int64 // should be 1, not 0, when starting from height 1

	// LastBlockHeight=0 at genesis (ie. block(H=0) does not exist)
	LastBlockHeight int64
	LastBlockID     types.BlockID
	LastBlockTime   time.Time

	// LastStateID contains App Hash and Height from previous state (at height-1)
	LastStateID types.StateID

	// Last Chain Lock is the last known chain locked height in consensus
	// It does not go to 0 if a block had no chain lock and should stay the same as the previous block
	LastCoreChainLockedBlockHeight uint32

	// LastValidators is used to validate block.LastPrecommits.
	// Validators are persisted to the database separately every time they change,
	// so we can query for historical validator sets.
	// Note that if s.LastBlockHeight causes a valset change,
	// we set s.LastHeightValidatorsChanged = s.LastBlockHeight + 1 + 1
	// Extra +1 due to nextValSet delay.
	Validators                  *types.ValidatorSet
	LastValidators              *types.ValidatorSet
	LastHeightValidatorsChanged int64

	// Consensus parameters used for validating blocks.
	// Changes returned by FinalizeBlock and updated after Commit.
	ConsensusParams                  types.ConsensusParams
	LastHeightConsensusParamsChanged int64

	// Merkle root of the results from executing prev block
	LastResultsHash tmbytes.HexBytes

	// the latest AppHash we've received from calling abci.Commit()
	// TODO: Rename to LastAppHash
	AppHash tmbytes.HexBytes
}

//  NewRound changes the State to apply settings new round and height to it.
// func (state *State) NewRound(ctx context.Context, height int64, round int32, lastBockHeader types.Header) error {
// 	if state.LastBlockHeight+1 != height {
// 		return fmt.Errorf("New height %d is in future, last block height was %d", height, state.LastBlockHeight)
// 	}

// 	if height == state.NextBlockSettings.Height {
// 		state.NextBlockSettings.Apply(height, state)
// 	}
// }

// Copy makes a copy of the State for mutating.
func (state State) Copy() State {
	return State{
		Version:       state.Version,
		ChainID:       state.ChainID,
		InitialHeight: state.InitialHeight,

		LastBlockHeight: state.LastBlockHeight,
		LastBlockID:     state.LastBlockID,
		LastBlockTime:   state.LastBlockTime,

		LastStateID: state.LastStateID.Copy(),

		LastCoreChainLockedBlockHeight: state.LastCoreChainLockedBlockHeight,

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
func (state State) Equals(state2 State) (bool, error) {
	sbz, err := state.Bytes()
	if err != nil {
		return false, err
	}
	s2bz, err := state2.Bytes()
	if err != nil {
		return false, err
	}
	return bytes.Equal(sbz, s2bz), nil
}

// Bytes serializes the State using protobuf, propagating marshaling
// errors
func (state State) Bytes() ([]byte, error) {
	sm, err := state.ToProto()
	if err != nil {
		return nil, err
	}
	bz, err := proto.Marshal(sm)
	if err != nil {
		return nil, err
	}
	return bz, nil
}

// IsEmpty returns true if the State is equal to the empty State.
func (state State) IsEmpty() bool {
	return state.Validators == nil // XXX can't compare to Empty
}

// ToProto takes the local state type and returns the equivalent proto type
func (state *State) ToProto() (*tmstate.State, error) {
	if state == nil {
		return nil, errors.New("state is nil")
	}

	sm := new(tmstate.State)

	sm.Version = state.Version.ToProto()
	sm.ChainID = state.ChainID
	sm.InitialHeight = state.InitialHeight
	sm.LastBlockHeight = state.LastBlockHeight

	sm.LastCoreChainLockedBlockHeight = state.LastCoreChainLockedBlockHeight

	sm.LastBlockID = state.LastBlockID.ToProto()
	sm.LastBlockTime = state.LastBlockTime
	vals, err := state.Validators.ToProto()
	if err != nil {
		return nil, err
	}
	sm.Validators = vals

	sm.LastStateID = state.LastStateID.ToProto()

	if state.LastBlockHeight >= 1 { // At Block 1 LastValidators is nil
		lVals, err := state.LastValidators.ToProto()
		if err != nil {
			return nil, err
		}
		sm.LastValidators = lVals
	}

	sm.LastHeightValidatorsChanged = state.LastHeightValidatorsChanged
	sm.ConsensusParams = state.ConsensusParams.ToProto()
	sm.LastHeightConsensusParamsChanged = state.LastHeightConsensusParamsChanged
	sm.LastResultsHash = state.LastResultsHash
	sm.AppHash = state.AppHash

	return sm, nil
}

// FromProto takes a state proto message & returns the local state type
func FromProto(pb *tmstate.State) (*State, error) { //nolint:golint
	if pb == nil {
		return nil, errors.New("nil State")
	}

	state := new(State)

	state.Version = VersionFromProto(pb.Version)
	state.ChainID = pb.ChainID
	state.InitialHeight = pb.InitialHeight

	bi, err := types.BlockIDFromProto(&pb.LastBlockID)
	if err != nil {
		return nil, err
	}
	state.LastBlockID = *bi
	state.LastBlockHeight = pb.LastBlockHeight
	state.LastBlockTime = pb.LastBlockTime

	si, err := types.StateIDFromProto(&pb.LastStateID)
	if err != nil {
		return nil, err
	}

	state.LastStateID = *si

	state.LastCoreChainLockedBlockHeight = pb.LastCoreChainLockedBlockHeight

	vals, err := types.ValidatorSetFromProto(pb.Validators)
	if err != nil {
		return nil, err
	}
	state.Validators = vals

	if state.LastBlockHeight >= 1 { // At Block 1 LastValidators is nil
		lVals, err := types.ValidatorSetFromProto(pb.LastValidators)
		if err != nil {
			return nil, err
		}
		state.LastValidators = lVals
	} else {
		state.LastValidators = types.NewEmptyValidatorSet()
	}

	state.LastHeightValidatorsChanged = pb.LastHeightValidatorsChanged
	state.ConsensusParams = types.ConsensusParamsFromProto(pb.ConsensusParams)
	state.LastHeightConsensusParamsChanged = pb.LastHeightConsensusParamsChanged
	state.LastResultsHash = pb.LastResultsHash
	state.AppHash = pb.AppHash

	return state, nil
}

//------------------------------------------------------------------------
// Create a block from the latest state

// MakeBlock builds a block from the current state with the given txs, commit,
// and evidence. Note it also takes a proposerProTxHash because the state does not
// track rounds, and hence does not know the correct proposer. TODO: fix this!
func (state State) MakeBlock(
	height int64,
	txs []types.Tx,
	commit *types.Commit,
	evidence []types.Evidence,
	proposerProTxHash types.ProTxHash,
	proposedAppVersion uint64,
) *types.Block {

	// Build base block with block data.
	block := types.MakeBlock(height, txs, commit, evidence)

	// Fill rest of header with state data.
	validatorsHash := state.Validators.Hash()
	block.Header.Populate(
		state.Version.Consensus, state.ChainID,
		tmtime.Now(), state.LastBlockID,
		validatorsHash, validatorsHash,
		state.ConsensusParams.HashConsensusParams(),
		state.AppHash,
		state.LastResultsHash,
		proposerProTxHash,
		proposedAppVersion,
	)

	return block
}

func (state State) ValidatorsAtHeight(height int64) *types.ValidatorSet {
	switch {
	case state.LastBlockHeight == height:
		return state.LastValidators
	default:
		return state.Validators
	}
}

// NewStateChangeset returns a structure that will hold new changes to the state, that can be applied once the block is finalized
func (state State) NewStateChangeset(ctx context.Context, proposalResponse proto.Message) (CurrentRoundState, error) {
	ret := CurrentRoundState{}
	err := ret.populate(ctx, proposalResponse, state)
	return ret, err
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
	genDocJSON, err := os.ReadFile(genDocFile)
	if err != nil {
		return nil, fmt.Errorf("couldn't read GenesisDoc file: %w", err)
	}
	genDoc, err := types.GenesisDocFromJSON(genDocJSON)
	if err != nil {
		return nil, fmt.Errorf("error reading GenesisDoc: %w", err)
	}
	return genDoc, nil
}

// MakeGenesisState creates state from types.GenesisDoc.
func MakeGenesisState(genDoc *types.GenesisDoc) (State, error) {
	err := genDoc.ValidateAndComplete()
	if err != nil {
		return State{}, fmt.Errorf("error in genesis doc: %w", err)
	}

	var validatorSet *types.ValidatorSet
	if genDoc.Validators == nil || len(genDoc.Validators) == 0 {
		validatorSet = types.NewValidatorSet(nil, nil, genDoc.QuorumType, nil, false)
	} else {
		validators := make([]*types.Validator, len(genDoc.Validators))
		hasAllPublicKeys := true
		for i, val := range genDoc.Validators {
			validators[i] = types.NewValidatorDefaultVotingPower(val.PubKey, val.ProTxHash)
			if val.PubKey == nil {
				hasAllPublicKeys = false
			}
		}
		validatorSet = types.NewValidatorSet(
			validators, genDoc.ThresholdPublicKey, genDoc.QuorumType, genDoc.QuorumHash, hasAllPublicKeys,
		)
	}

	stateID := types.StateID{
		Height:  genDoc.InitialHeight,
		AppHash: genDoc.AppHash,
	}

	return State{
		Version:       InitStateVersion,
		ChainID:       genDoc.ChainID,
		InitialHeight: genDoc.InitialHeight,

		LastBlockHeight: 0,
		LastBlockID:     types.BlockID{},
		LastStateID:     stateID,
		LastBlockTime:   genDoc.GenesisTime,

		LastCoreChainLockedBlockHeight: genDoc.InitialCoreChainLockedHeight,

		Validators: validatorSet,
		// The quorum type must be 0 on an empty validator set
		LastValidators:              types.NewEmptyValidatorSet(),
		LastHeightValidatorsChanged: genDoc.InitialHeight,

		ConsensusParams:                  *genDoc.ConsensusParams,
		LastHeightConsensusParamsChanged: genDoc.InitialHeight,

		AppHash: genDoc.AppHash,
	}, nil
}
