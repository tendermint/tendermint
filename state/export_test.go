package state

import (
	abci "github.com/tendermint/tendermint/abci/types"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/types"
)

const ValSetCheckpointInterval = valSetCheckpointInterval

// UpdateState is an alias for updateState exported from execution.go,
// exclusively and explicitly for testing. TODO: Remove.
func UpdateState(
	state State,
	blockID types.BlockID,
	header *types.Header,
	abciResponses *ABCIResponses,
	validatorUpdates []*types.Validator,
) (State, error) {
	return updateState(state, blockID, header, abciResponses, validatorUpdates)
}

// ValidateValidatorUpdates is an alias for validateValidatorUpdates exported
// from execution.go, exclusively and explicitly for testing. TODO: Remove.
func ValidateValidatorUpdates(abciUpdates []abci.ValidatorUpdate, params types.ValidatorParams) error {
	return validateValidatorUpdates(abciUpdates, params)
}

// CalcValidatorsKey is an alias for the private calcValidatorsKey method in
// store.go, exported exclusively and explicitly for testing. TODO: Remove.
func CalcValidatorsKey(height int64) []byte {
	return calcValidatorsKey(height)
}

// SaveABCIResponses is an alias for the private saveABCIResponses method in
// store.go, exported exclusively and explicitly for testing. TODO: Remove.
func SaveABCIResponses(db dbm.DB, height int64, abciResponses *ABCIResponses) {
	saveABCIResponses(db, height, abciResponses)
}

// SaveConsensusParamsInfo is an alias for the private saveConsensusParamsInfo
// method in store.go, exported exclusively and explicitly for testing.
// TODO: Remove.
func SaveConsensusParamsInfo(db dbm.DB, nextHeight, changeHeight int64, params types.ConsensusParams) {
	saveConsensusParamsInfo(db, nextHeight, changeHeight, params)
}

// SaveValidatorsInfo is an alias for the private saveValidatorsInfo method in
// store.go, exported exclusively and explicitly for testing. TODO: Remove.
func SaveValidatorsInfo(db dbm.DB, height, lastHeightChanged int64, valSet *types.ValidatorSet) {
	saveValidatorsInfo(db, height, lastHeightChanged, valSet)
}
