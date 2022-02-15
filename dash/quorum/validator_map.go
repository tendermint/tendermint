package quorum

import (
	"sort"
	"strings"

	"github.com/tendermint/tendermint/types"
)

// validatorMapIndexType represents data that is used to index `validatorMap` elements
type validatorMapIndexType string

// validatorMap maps validator ID to the validator
type validatorMap map[validatorMapIndexType]types.Validator

// validatorMapIndex returns index value to use inside validator map
func validatorMapIndex(v types.Validator) validatorMapIndexType {
	return validatorMapIndexType(v.ProTxHash.String())
}

// newValidatorMap creates a new validatoMap based on a slice of Validators
func newValidatorMap(validators []*types.Validator) validatorMap {
	newMap := make(validatorMap, len(validators))
	for _, validator := range validators {
		if !validator.NodeAddress.Zero() {
			newMap[validatorMapIndex(*validator)] = *validator
		}
	}
	return newMap
}

// values returns content (values) of the map as a slice
func (vm validatorMap) values() []*types.Validator {
	vals := make([]*types.Validator, 0, len(vm))
	for _, v := range vm {
		vals = append(vals, v.Copy())
	}
	return vals
}

// contains returns true if the validatorMap contains `What`, false otherwise.
// Items are compared using node ID.
func (vm validatorMap) contains(what types.Validator) bool {
	_, ok := vm[validatorMapIndex(what)]
	return ok
}

// URIs returns URIs of all validators in this map
func (vm validatorMap) NodeIDs() []string {
	uris := make([]string, 0, len(vm))
	for _, v := range vm {
		uris = append(uris, string(v.NodeAddress.NodeID))
	}
	return uris
}

func (vm validatorMap) String() string {
	resp := make(sort.StringSlice, 0, len(vm))
	for _, v := range vm {
		resp = append(resp, v.String())
	}
	resp.Sort()
	return strings.Join(resp, "\n")
}
