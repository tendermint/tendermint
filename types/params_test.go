package types

import (
	"bytes"
	"sort"
	"testing"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"

	"github.com/stretchr/testify/assert"
	abci "github.com/tendermint/tendermint/abci/types"
)

var (
	valEd25519   = []string{ed25519.PubKeyAminoRoute}
	valSecp256k1 = []string{secp256k1.PubKeyAminoRoute}
)

func TestConsensusParamsValidation(t *testing.T) {
	testCases := []struct {
		params ConsensusParams
		valid  bool
	}{
		// test block size
		0: {makeParams(1, 0, 1, valEd25519), true},
		1: {makeParams(0, 0, 1, valEd25519), false},
		2: {makeParams(47*1024*1024, 0, 1, valEd25519), true},
		3: {makeParams(10, 0, 1, valEd25519), true},
		4: {makeParams(100*1024*1024, 0, 1, valEd25519), true},
		5: {makeParams(101*1024*1024, 0, 1, valEd25519), false},
		6: {makeParams(1024*1024*1024, 0, 1, valEd25519), false},
		7: {makeParams(1024*1024*1024, 0, -1, valEd25519), false},
		// test evidence age
		8: {makeParams(1, 0, 0, valEd25519), false},
		9: {makeParams(1, 0, -1, valEd25519), false},
		// test no pubkey type provided
		10: {makeParams(1, 0, 1, []string{}), false},
	}
	for i, tc := range testCases {
		if tc.valid {
			assert.NoErrorf(t, tc.params.Validate(), "expected no error for valid params (#%d)", i)
		} else {
			assert.Errorf(t, tc.params.Validate(), "expected error for non valid params (#%d)", i)
		}
	}
}

func makeParams(blockBytes, blockGas, evidenceAge int64, pubkeyTypes []string) ConsensusParams {
	return ConsensusParams{
		BlockSize: BlockSize{
			MaxBytes: blockBytes,
			MaxGas:   blockGas,
		},
		EvidenceParams: EvidenceParams{
			MaxAge: evidenceAge,
		},
		Validator: ValidatorParams{
			ValidatorPubkeyTypes: pubkeyTypes,
		},
	}
}

func TestConsensusParamsHash(t *testing.T) {
	params := []ConsensusParams{
		makeParams(4, 2, 3, valEd25519),
		makeParams(1, 4, 3, valEd25519),
		makeParams(1, 2, 4, valEd25519),
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
			makeParams(1, 2, 3, valEd25519),
			&abci.ConsensusParams{},
			makeParams(1, 2, 3, valEd25519),
		},
		// fine updates
		{
			makeParams(1, 2, 3, valEd25519),
			&abci.ConsensusParams{
				BlockSize: &abci.BlockSize{
					MaxBytes: 100,
					MaxGas:   200,
				},
				EvidenceParams: &abci.EvidenceParams{
					MaxAge: 300,
				},
				ValidatorParams: &abci.ValidatorParams{
					ValidatorPubkeyTypes: valSecp256k1,
				},
			},
			makeParams(100, 200, 300, valSecp256k1),
		},
	}
	for _, tc := range testCases {
		assert.Equal(t, tc.updatedParams, tc.params.Update(tc.updates))
	}
}
