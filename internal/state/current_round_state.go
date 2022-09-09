package state

import (
	"bytes"
	"context"
	"fmt"

	"github.com/gogo/protobuf/proto"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/dash"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmtypes "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

const (
	prepareProposal = "ResponsePrepareProposal"
	processProposal = "ResponseProcessProposal"
)

// CurrentRoundState ...
type CurrentRoundState struct {
	// Base state for the changes
	Base State

	// AppHash of current block
	AppHash tmbytes.HexBytes `json:"app_hash"`

	// TxResults for current block
	TxResults []*abci.ExecTxResult `json:"tx_results"`
	// ResultsHash of current block
	ResultsHash []byte `json:"results_hash"`

	CoreChainLock *types.CoreChainLock

	// Items changed in next block

	NextConsensusParams              types.ConsensusParams
	LastHeightConsensusParamsChanged int64

	NextValidators              *types.ValidatorSet
	LastHeightValidatorsChanged int64

	// responseType points to responseType of state changes - prepareProposal, processProposal or empty string for nil
	responseType string

	// response stores process proposal response, received from ABCI app or converted from PrepareProposal
	response abci.ResponseProcessProposal
}

func (candidate CurrentRoundState) IsEmpty() bool {
	return candidate.AppHash == nil
}

// UpdateBlock changes block fields to reflect the ones returned in PrepareProposal / ProcessProposal
func (candidate CurrentRoundState) UpdateBlock(target *types.Block) error {
	if candidate.responseType != prepareProposal {
		return fmt.Errorf("block can be updated only based on '%s' response, got '%s'", processProposal, candidate.responseType)
	}
	target.AppHash = candidate.AppHash
	target.ResultsHash = candidate.ResultsHash
	target.NextValidatorsHash = candidate.NextValidators.Hash()
	target.CoreChainLock = candidate.CoreChainLock
	if candidate.CoreChainLock != nil {
		target.CoreChainLockedHeight = candidate.CoreChainLock.CoreBlockHeight
	} else {
		target.CoreChainLockedHeight = candidate.Base.LastCoreChainLockedBlockHeight
	}
	return nil
}

// UpdateState updates state when the block is committed. State will contain data needed by next block.
func (candidate CurrentRoundState) UpdateState(ctx context.Context, target *State) error {
	target.AppHash = candidate.AppHash
	target.LastResultsHash = candidate.ResultsHash
	target.ConsensusParams = candidate.NextConsensusParams
	target.LastHeightConsensusParamsChanged = candidate.LastHeightConsensusParamsChanged
	target.Version.Consensus.App = candidate.NextConsensusParams.Version.AppVersion
	target.Validators = candidate.NextValidators
	target.LastHeightValidatorsChanged = candidate.LastHeightValidatorsChanged
	if candidate.CoreChainLock != nil {
		target.LastCoreChainLockedBlockHeight = candidate.CoreChainLock.CoreBlockHeight
	}
	return nil
}

// UpdateFunc implements UpdateFunc
func (candidate CurrentRoundState) UpdateFunc(ctx context.Context, state State) (State, error) {
	err := candidate.UpdateState(ctx, &state)
	return state, err
}

func (candidate *CurrentRoundState) populate(ctx context.Context, proposalResponse proto.Message, baseState State) error {
	switch resp := proposalResponse.(type) {
	case *abci.ResponsePrepareProposal:
		candidate.responseType = prepareProposal
		candidate.response = resp.ToResponseProcessProposal()

	case *abci.ResponseProcessProposal:
		if !resp.IsAccepted() {
			return fmt.Errorf("proposal not accepted by abci app: %s", resp.Status)
		}
		candidate.responseType = processProposal
		candidate.response = *resp

	case nil: // Assuming no changes
		return candidate.update(ctx, baseState, nil, nil, nil, nil, nil)

	default:
		return fmt.Errorf("unsupported response type %T", resp)
	}

	return candidate.update(
		ctx,
		baseState,
		candidate.response.AppHash,
		candidate.response.TxResults,
		candidate.response.ConsensusParamUpdates,
		candidate.response.ValidatorSetUpdate,
		candidate.response.CoreChainLockUpdate,
	)
}

func (candidate *CurrentRoundState) update(
	ctx context.Context,
	baseState State,
	appHash tmbytes.HexBytes,
	txResults []*abci.ExecTxResult,
	consensusParamUpdates *tmtypes.ConsensusParams,
	validatorSetUpdate *abci.ValidatorSetUpdate,
	coreChainLockUpdate *tmtypes.CoreChainLock,
) error {
	candidate.Base = baseState
	candidate.AppHash = appHash.Copy()

	if err := candidate.populateTxResults(txResults); err != nil {
		return err
	}
	// Consensus params need to be populated before validators
	if err := candidate.populateConsensusParams(consensusParamUpdates); err != nil {
		return err
	}
	if err := candidate.populateValsetUpdates(ctx, validatorSetUpdate); err != nil {
		return err
	}
	if err := candidate.populateChainlock(coreChainLockUpdate); err != nil {
		return err
	}

	return nil
}

func (candidate CurrentRoundState) StateID() types.StateID {
	var appHash tmbytes.HexBytes
	if len(candidate.AppHash) > 0 {
		appHash = candidate.AppHash.Copy()
	} else {
		appHash = make([]byte, crypto.DefaultAppHashSize)
	}

	return types.StateID{
		Height:  candidate.GetHeight(),
		AppHash: appHash,
	}
}

// GetHeight returns height of current block
func (candidate CurrentRoundState) GetHeight() int64 {
	if candidate.Base.LastBlockHeight == 0 {
		return candidate.Base.InitialHeight
	}

	return candidate.Base.LastBlockHeight + 1
}

func (candidate CurrentRoundState) GetProcessProposalResponse() abci.ResponseProcessProposal {
	return candidate.response
}

func (candidate *CurrentRoundState) populateTxResults(txResults []*abci.ExecTxResult) error {
	hash, err := abci.TxResultsHash(txResults)
	if err != nil {
		return fmt.Errorf("marshaling TxResults: %w", err)
	}
	candidate.ResultsHash = hash
	candidate.TxResults = txResults

	return nil
}

func (candidate *CurrentRoundState) populateChainlock(chainlockProto *tmtypes.CoreChainLock) error {
	chainlock, err := types.CoreChainLockFromProto(chainlockProto)
	if err != nil {
		return err
	}
	lastChainlockHeight := candidate.Base.LastCoreChainLockedBlockHeight

	if chainlock == nil || (chainlock.CoreBlockHeight <= lastChainlockHeight) {
		candidate.CoreChainLock = nil
		return nil
	}

	candidate.CoreChainLock = chainlock
	return nil
}

// populateConsensusParams updates ConsensusParams, Version and LastHeightConsensusParamsChanged
func (candidate *CurrentRoundState) populateConsensusParams(updates *tmtypes.ConsensusParams) error {

	if updates == nil || updates.Equal(&tmtypes.ConsensusParams{}) {
		candidate.NextConsensusParams = candidate.Base.ConsensusParams
		candidate.LastHeightConsensusParamsChanged = candidate.Base.LastHeightConsensusParamsChanged
		return nil
	}

	current := candidate.NextConsensusParams
	if current.IsZero() {
		current = candidate.Base.ConsensusParams
	}

	// NOTE: must not mutate state.ConsensusParams
	nextParams := current.UpdateConsensusParams(updates)
	err := nextParams.ValidateConsensusParams()
	if err != nil {
		return fmt.Errorf("error updating consensus params: %w", err)
	}
	candidate.NextConsensusParams = nextParams

	// Change results from this height but only applies to the next height.
	candidate.LastHeightConsensusParamsChanged = candidate.GetHeight() + 1

	return nil
}

// populateValsetUpdates calculates and populates Validators and LastHeightValidatorsChanged
// CONTRACT: candidate.ConsensusParams were already populated
func (candidate *CurrentRoundState) populateValsetUpdates(ctx context.Context, update *abci.ValidatorSetUpdate) error {
	base := candidate.Base

	newValSet, err := valsetUpdate(ctx, update, base.Validators, candidate.NextConsensusParams.Validator)
	if err != nil {
		return fmt.Errorf("validator set updates: %w", err)
	}
	newValSet.IncrementProposerPriority(1)
	candidate.NextValidators = newValSet

	if update != nil && len(update.ValidatorUpdates) > 0 {
		// Change results from this height but only applies to the next height.
		candidate.LastHeightValidatorsChanged = candidate.GetHeight() + 1
	} else {
		candidate.LastHeightValidatorsChanged = base.LastHeightValidatorsChanged
	}

	return nil
}

// valsetUpdate processes validator set updates received from ABCI app.
func valsetUpdate(
	ctx context.Context,
	vu *abci.ValidatorSetUpdate,
	currentVals *types.ValidatorSet,
	params types.ValidatorParams,
) (*types.ValidatorSet, error) {
	err := validateValidatorSetUpdate(vu, params)
	if err != nil {
		return nil, fmt.Errorf("validating validator updates: %w", err)
	}

	validatorUpdates, thresholdPubKey, quorumHash, err := types.PB2TM.ValidatorUpdatesFromValidatorSet(vu)
	if err != nil {
		return nil, fmt.Errorf("converting validator updates to native types: %w", err)
	}
	// Copy the valset so we can apply changes from FinalizeBlock
	// and update s.LastValidators and s.Validators.
	nValSet := currentVals.Copy()
	// Update the validator set with the latest abciResponses.
	if len(validatorUpdates) > 0 {
		if bytes.Equal(nValSet.QuorumHash, quorumHash) {
			err = nValSet.UpdateWithChangeSet(validatorUpdates, thresholdPubKey, quorumHash)
			if err != nil {
				return nil, err
			}
		} else {
			nodeProTxHash, _ := dash.ProTxHashFromContext(ctx)
			// if we don't have proTxHash, NewValidatorSetWithLocalNodeProTxHash behaves like NewValidatorSet
			nValSet = types.NewValidatorSetWithLocalNodeProTxHash(validatorUpdates, thresholdPubKey,
				currentVals.QuorumType, quorumHash, nodeProTxHash)
		}
	}
	return nValSet, nil
}
