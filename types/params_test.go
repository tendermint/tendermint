package types

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	abci "github.com/tendermint/tendermint/abci/types"
)

var (
	valEd25519   = []string{ABCIPubKeyTypeEd25519}
	valSecp256k1 = []string{ABCIPubKeyTypeSecp256k1}
)

func TestConsensusParamsValidation(t *testing.T) {
	testCases := []struct {
		params ConsensusParams
		valid  bool
	}{
		// test block params
		0: {makeParams(1, 0, 10, 1, valEd25519), true},
		1: {makeParams(0, 0, 10, 1, valEd25519), false},
		2: {makeParams(47*1024*1024, 0, 10, 1, valEd25519), true},
		3: {makeParams(10, 0, 10, 1, valEd25519), true},
		4: {makeParams(100*1024*1024, 0, 10, 1, valEd25519), true},
		5: {makeParams(101*1024*1024, 0, 10, 1, valEd25519), false},
		6: {makeParams(1024*1024*1024, 0, 10, 1, valEd25519), false},
		7: {makeParams(1024*1024*1024, 0, 10, -1, valEd25519), false},
		8: {makeParams(1, 0, -10, 1, valEd25519), false},
		// test evidence params
		9:  {makeParams(1, 0, 10, 0, valEd25519), false},
		10: {makeParams(1, 0, 10, -1, valEd25519), false},
		// test no pubkey type provided
		11: {makeParams(1, 0, 10, 1, []string{}), false},
		// test invalid pubkey type provided
		12: {makeParams(1, 0, 10, 1, []string{"potatoes make good pubkeys"}), false},
	}
	for i, tc := range testCases {
		if tc.valid {
			assert.NoErrorf(t, tc.params.Validate(), "expected no error for valid params (#%d)", i)
		} else {
			assert.Errorf(t, tc.params.Validate(), "expected error for non valid params (#%d)", i)
		}
	}
}

func makeParams(
	blockBytes, blockGas int64,
	blockTimeIotaMs int64,
	evidenceAge int64,
	pubkeyTypes []string,
) ConsensusParams {
	return ConsensusParams{
		Block: BlockParams{
			MaxBytes:   blockBytes,
			MaxGas:     blockGas,
			TimeIotaMs: blockTimeIotaMs,
		},
		Evidence: EvidenceParams{
			MaxAge: evidenceAge,
		},
		Validator: ValidatorParams{
			PubKeyTypes: pubkeyTypes,
		},
	}
}

func TestConsensusParamsHash(t *testing.T) {
	params := []ConsensusParams{
		makeParams(4, 2, 10, 3, valEd25519),
		makeParams(1, 4, 10, 3, valEd25519),
		makeParams(1, 2, 10, 4, valEd25519),
		makeParams(2, 5, 10, 7, valEd25519),
		makeParams(1, 7, 10, 6, valEd25519),
		makeParams(9, 5, 10, 4, valEd25519),
		makeParams(7, 8, 10, 9, valEd25519),
		makeParams(4, 6, 10, 5, valEd25519),
	}

	hashes := make([][]byte, len(params))
	for i := range params {
		hashes[i] = params[i].Hash()
	}

	// make sure there are no duplicates...
	// sort, then check in order for matches
	sort.Slice(hashes, func(i, j int) bool {
		return bytes.Compare(hashes[i], hashes[j]) < 0
	})
	for i := 0; i < len(hashes)-1; i++ {
		assert.NotEqual(t, hashes[i], hashes[i+1])
	}
}

func TestConsensusParamsUpdate(t *testing.T) {
	testCases := []struct {
		params        ConsensusParams
		updates       *abci.ConsensusParams
		updatedParams ConsensusParams
	}{
		// empty updates
		{
			makeParams(1, 2, 10, 3, valEd25519),
			&abci.ConsensusParams{},
			makeParams(1, 2, 10, 3, valEd25519),
		},
		// fine updates
		{
			makeParams(1, 2, 10, 3, valEd25519),
			&abci.ConsensusParams{
				Block: &abci.BlockParams{
					MaxBytes: 100,
					MaxGas:   200,
				},
				Evidence: &abci.EvidenceParams{
					MaxAge: 300,
				},
				Validator: &abci.ValidatorParams{
					PubKeyTypes: valSecp256k1,
				},
			},
			makeParams(100, 200, 10, 300, valSecp256k1),
		},
	}
	for _, tc := range testCases {
		assert.Equal(t, tc.updatedParams, tc.params.Update(tc.updates))
	}
}
