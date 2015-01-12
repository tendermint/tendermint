package rpc

import (
	"net/http"

	state_ "github.com/tendermint/tendermint/state"
)

func ListValidatorsHandler(w http.ResponseWriter, r *http.Request) {
	var blockHeight uint
	var bondedValidators []*state_.Validator
	var unbondingValidators []*state_.Validator

	state := consensusState.GetState()
	blockHeight = state.LastBlockHeight
	state.BondedValidators.Iterate(func(index uint, val *state_.Validator) bool {
		bondedValidators = append(bondedValidators, val)
		return false
	})
	state.UnbondingValidators.Iterate(func(index uint, val *state_.Validator) bool {
		unbondingValidators = append(unbondingValidators, val)
		return false
	})

	WriteAPIResponse(w, API_OK, struct {
		BlockHeight         uint
		BondedValidators    []*state_.Validator
		UnbondingValidators []*state_.Validator
	}{blockHeight, bondedValidators, unbondingValidators})
}
