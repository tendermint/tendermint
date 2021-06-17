package state

import (
	"github.com/tendermint/tendermint/crypto"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

//
// TODO: Remove dependence on all entities exported from this file.
//
// Every entity exported here is dependent on a private entity from the `state`
// package. Currently, these functions are only made available to tests in the
// `state_test` package, but we should not be relying on them for our testing.
// Instead, we should be exclusively relying on exported entities for our
// testing, and should be refactoring exported entities to make them more
// easily testable from outside of the package.
//

const ValSetCheckpointInterval = valSetCheckpointInterval

// UpdateState is an alias for updateState exported from execution.go,
// exclusively and explicitly for testing.
func UpdateState(
	state State,
	blockID types.BlockID,
	header *types.Header,
	abciResponses *tmstate.ABCIResponses,
	validatorUpdates []*types.Validator,
	newThresholdPublicKey crypto.PubKey,
	quorumHash crypto.QuorumHash,
) (State, error) {
	return updateState(state, blockID, header, abciResponses, validatorUpdates, newThresholdPublicKey, quorumHash)
}

// ValidateValidatorUpdates is an alias for validateValidatorUpdates exported
// from execution.go, exclusively and explicitly for testing.
func ValidateValidatorUpdates(abciUpdates []abci.ValidatorUpdate, params tmproto.ValidatorParams) error {
	return validateValidatorUpdates(abciUpdates, params)
}

// SaveValidatorsInfo is an alias for the private saveValidatorsInfo method in
// store.go, exported exclusively and explicitly for testing.
func SaveValidatorsInfo(db dbm.DB, height, lastHeightChanged int64, valSet *types.ValidatorSet) error {
	stateStore := dbStore{db}
	return stateStore.saveValidatorsInfo(height, lastHeightChanged, valSet)
}
