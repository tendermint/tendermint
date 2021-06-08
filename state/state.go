package state

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/tendermint/tendermint/crypto"
	"io/ioutil"
	"time"

	"github.com/gogo/protobuf/proto"

	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
	"github.com/tendermint/tendermint/version"
)

// database keys
var (
	stateKey = []byte("stateKey")
)

//-----------------------------------------------------------------------------

// InitStateVersion sets the Consensus.Block and Software versions,
// but leaves the Consensus.App version blank.
// The Consensus.App version will be set during the Handshake, once
// we hear from the app what protocol version it is running.
var InitStateVersion = tmstate.Version{
	Consensus: tmversion.Consensus{
		Block: version.BlockProtocol,
		App:   0,
	},
	Software: version.TMCoreSemVer,
}

//-----------------------------------------------------------------------------

// State is a short description of the latest committed block of the Tendermint consensus.
// It keeps all information necessary to validate new blocks,
// including the last validator set and the consensus params.
// All fields are exposed so the struct can be easily serialized,
// but none of them should be mutated directly.
// Instead, use state.Copy() or state.NextState(...).
// NOTE: not goroutine-safe.
type State struct {
	Version tmstate.Version

	// immutable
	ChainID       string
	NodeProTxHash *crypto.ProTxHash //there might not be a nodeProTxHash
	InitialHeight int64 // should be 1, not 0, when starting from height 1

	// LastBlockHeight=0 at genesis (ie. block(H=0) does not exist)
	LastBlockHeight int64
	LastBlockID     types.BlockID
	LastBlockTime   time.Time

	// The Last StateID is actually the previous App Hash
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
	NextValidators              *types.ValidatorSet
	Validators                  *types.ValidatorSet
	LastValidators              *types.ValidatorSet
	LastHeightValidatorsChanged int64

	// Consensus parameters used for validating blocks.
	// Changes returned by EndBlock and updated after Commit.
	ConsensusParams                  tmproto.ConsensusParams
	LastHeightConsensusParamsChanged int64

	// Merkle root of the results from executing prev block
	LastResultsHash []byte

	// the latest AppHash we've received from calling abci.Commit()
	AppHash []byte
}

// Copy makes a copy of the State for mutating.
func (state State) Copy() State {

	return State{
		Version:       state.Version,
		ChainID:       state.ChainID,
		InitialHeight: state.InitialHeight,

		LastBlockHeight: state.LastBlockHeight,
		LastBlockID:     state.LastBlockID,
		LastBlockTime:   state.LastBlockTime,

		LastStateID: state.LastStateID,

		LastCoreChainLockedBlockHeight: state.LastCoreChainLockedBlockHeight,

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

// Bytes serializes the State using protobuf.
// It panics if either casting to protobuf or serialization fails.
func (state State) Bytes() []byte {
	sm, err := state.ToProto()
	if err != nil {
		panic(err)
	}
	bz, err := proto.Marshal(sm)
	if err != nil {
		panic(err)
	}
	return bz
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

	sm.Version = state.Version
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

	nVals, err := state.NextValidators.ToProto()
	if err != nil {
		return nil, err
	}
	sm.NextValidators = nVals

	if state.LastBlockHeight >= 1 { // At Block 1 LastValidators is nil
		lVals, err := state.LastValidators.ToProto()
		if err != nil {
			return nil, err
		}
		sm.LastValidators = lVals
	}

	sm.LastHeightValidatorsChanged = state.LastHeightValidatorsChanged
	sm.ConsensusParams = state.ConsensusParams
	sm.LastHeightConsensusParamsChanged = state.LastHeightConsensusParamsChanged
	sm.LastResultsHash = state.LastResultsHash
	sm.AppHash = state.AppHash

	return sm, nil
}

// StateFromProto takes a state proto message & returns the local state type
func StateFromProto(pb *tmstate.State) (*State, error) { //nolint:golint
	if pb == nil {
		return nil, errors.New("nil State")
	}

	state := new(State)

	state.Version = pb.Version
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

	nVals, err := types.ValidatorSetFromProto(pb.NextValidators)
	if err != nil {
		return nil, err
	}
	state.NextValidators = nVals

	if state.LastBlockHeight >= 1 { // At Block 1 LastValidators is nil
		lVals, err := types.ValidatorSetFromProto(pb.LastValidators)
		if err != nil {
			return nil, err
		}
		state.LastValidators = lVals
	} else {
		state.LastValidators = types.NewValidatorSet(nil, nil, 0, nil, false)
	}

	state.LastHeightValidatorsChanged = pb.LastHeightValidatorsChanged
	state.ConsensusParams = pb.ConsensusParams
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
	coreChainLock *types.CoreChainLock,
	txs []types.Tx,
	commit *types.Commit,
	evidence []types.Evidence,
	proposerProTxHash types.ProTxHash,
) (*types.Block, *types.PartSet) {

	var coreChainLockHeight uint32
	if coreChainLock == nil {
		coreChainLockHeight = state.LastCoreChainLockedBlockHeight
	} else {
		coreChainLockHeight = coreChainLock.CoreBlockHeight
	}

	// Build base block with block data.
	block := types.MakeBlock(height, coreChainLockHeight, coreChainLock, txs, commit, evidence)

	// Set time.
	var timestamp time.Time
	if height == state.InitialHeight {
		timestamp = state.LastBlockTime // genesis time
	} else {
		currentTime := tmtime.Now()
		if currentTime.Before(state.LastBlockTime) {
			// this is weird, propose last block time
			timestamp = state.LastBlockTime
		} else {
			timestamp = currentTime
		}
	}

	// Fill rest of header with state data.
	block.Header.Populate(
		state.Version.Consensus, state.ChainID,
		timestamp, state.LastBlockID,
		state.Validators.Hash(), state.NextValidators.Hash(),
		types.HashConsensusParams(state.ConsensusParams), state.AppHash, state.LastResultsHash,
		proposerProTxHash,
	)

	return block, block.MakePartSet(types.BlockPartSizeBytes)
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
		return nil, fmt.Errorf("couldn't read GenesisDoc file: %v", err)
	}
	genDoc, err := types.GenesisDocFromJSON(genDocJSON)
	if err != nil {
		return nil, fmt.Errorf("error reading GenesisDoc: %v", err)
	}
	return genDoc, nil
}

// MakeGenesisState creates state from types.GenesisDoc.
func MakeGenesisState(genDoc *types.GenesisDoc) (State, error) {
	err := genDoc.ValidateAndComplete()
	if err != nil {
		return State{}, fmt.Errorf("error in genesis file: %v", err)
	}

	var validatorSet, nextValidatorSet *types.ValidatorSet
	if genDoc.Validators == nil {
		validatorSet = types.NewValidatorSet(nil, nil, genDoc.QuorumType, nil, false)
		nextValidatorSet = types.NewValidatorSet(nil, nil, genDoc.QuorumType,nil, false)
	} else {
		validators := make([]*types.Validator, len(genDoc.Validators))
		hasAllPublicKeys := true
		for i, val := range genDoc.Validators {
			validators[i] = types.NewValidatorDefaultVotingPower(val.PubKey, val.ProTxHash)
			if val.PubKey == nil {
				hasAllPublicKeys = false
			}
		}
		validatorSet = types.NewValidatorSet(validators, genDoc.ThresholdPublicKey, genDoc.QuorumType, genDoc.QuorumHash, hasAllPublicKeys)
		nextValidatorSet = types.NewValidatorSet(validators, genDoc.ThresholdPublicKey, genDoc.QuorumType, genDoc.QuorumHash, hasAllPublicKeys).CopyIncrementProposerPriority(1)
	}

	return State{
		Version:       InitStateVersion,
		ChainID:       genDoc.ChainID,
		NodeProTxHash: genDoc.NodeProTxHash,
		InitialHeight: genDoc.InitialHeight,

		LastBlockHeight: 0,
		LastBlockID:     types.BlockID{},
		LastStateID:     types.StateID{},
		LastBlockTime:   genDoc.GenesisTime,

		LastCoreChainLockedBlockHeight: genDoc.InitialCoreChainLockedHeight,

		NextValidators:              nextValidatorSet,
		Validators:                  validatorSet,
		LastValidators:              types.NewValidatorSet(nil, nil, genDoc.QuorumType, nil, false),
		LastHeightValidatorsChanged: genDoc.InitialHeight,

		ConsensusParams:                  *genDoc.ConsensusParams,
		LastHeightConsensusParamsChanged: genDoc.InitialHeight,

		AppHash: genDoc.AppHash,
	}, nil
}
