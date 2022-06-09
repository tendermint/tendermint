// Package selectpeers is package contains algorithm that selects peers based on the deterministic connection
// selection algorithm described in DIP-6
package selectpeers

import (
	"fmt"
	"math"

	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/types"
)

// minValidators is a minimum number of validators needed in order to execute the selection
// algorithm. For less than this number, we connect to all validators.
const minValidators = 5

// DIP6 selector selects validators from the `validatorSetMembers`, based on algorithm
// described in DIP-6 https://github.com/dashpay/dips/blob/master/dip-0006.md
type dip6PeerSelector struct {
	quorumHash bytes.HexBytes
}

// NewDIP6ValidatorSelector creates new implementation of validator selector algorithm
func NewDIP6ValidatorSelector(quorumHash bytes.HexBytes) ValidatorSelector {
	return &dip6PeerSelector{quorumHash: quorumHash}
}

// SelectValidators implements ValidtorSelector.
// SelectValidators selects some validators from `validatorSetMembers`, according to the algorithm
// described in DIP-6 https://github.com/dashpay/dips/blob/master/dip-0006.md
func (s *dip6PeerSelector) SelectValidators(
	validatorSetMembers []*types.Validator,
	me *types.Validator,
) ([]*types.Validator, error) {
	if len(validatorSetMembers) < 2 {
		return nil, fmt.Errorf("not enough validators: got %d, need 2", len(validatorSetMembers))
	}
	// Build the deterministic list of quorum members:
	// 1. Retrieve the deterministic masternode list which is valid at quorumHeight
	// 2. Calculate SHA256(proTxHash, quorumHash) for each entry in the list
	// 3. Sort the resulting list by the calculated hashes
	sortedValidators := newSortedValidatorList(validatorSetMembers, s.quorumHash)

	// Loop through the list until the member finds itself in the list. The index at which it finds itself is called i.
	meSortable := newSortableValidator(*me, s.quorumHash)
	myIndex := sortedValidators.index(meSortable)
	if myIndex < 0 {
		return []*types.Validator{}, fmt.Errorf("current node is not a member of provided validator set")
	}

	// Fallback if we don't have enough validators, we connect to all of them
	if sortedValidators.Len() < minValidators {
		ret := make([]*types.Validator, 0, len(validatorSetMembers)-1)
		// We connect to all validators
		for index, val := range sortedValidators {
			if index != myIndex {
				ret = append(ret, val.Copy())
			}
		}
		return ret, nil
	}

	// Calculate indexes (i+2^k)%n where k is in the range 0..floor(log2(n-1))-1
	// and n is equal to the size of the list.
	n := float64(sortedValidators.Len())
	count := math.Floor(math.Log2(n-1.0)) - 1.0

	ret := make([]*types.Validator, 0, int(count))
	i := float64(myIndex)
	for k := float64(0); k <= count; k++ {
		index := int(math.Mod(i+math.Pow(2, k), n))
		// Add addresses of masternodes at indexes calculated at previous step
		// to the set of deterministic connections.
		ret = append(ret, sortedValidators[index].Validator.Copy())
	}

	return ret, nil
}
