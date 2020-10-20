package types

import (
	"bytes"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	abci "github.com/tendermint/tendermint/abci/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

var (
	valEd25519   = []string{ABCIPubKeyTypeEd25519}
	valSecp256k1 = []string{ABCIPubKeyTypeSecp256k1}
)

func TestConsensusParamsValidation(t *testing.T) {
	testCases := []struct {
		params tmproto.ConsensusParams
		valid  bool
	}{
		// test block params
		0: {makeParams(1, 0, 10, 2, 0, valEd25519), true},
		1: {makeParams(0, 0, 10, 2, 0, valEd25519), false},
		2: {makeParams(47*1024*1024, 0, 10, 2, 0, valEd25519), true},
		3: {makeParams(10, 0, 10, 2, 0, valEd25519), true},
		4: {makeParams(100*1024*1024, 0, 10, 2, 0, valEd25519), true},
		5: {makeParams(101*1024*1024, 0, 10, 2, 0, valEd25519), false},
		6: {makeParams(1024*1024*1024, 0, 10, 2, 0, valEd25519), false},
		7: {makeParams(1024*1024*1024, 0, 10, -1, 0, valEd25519), false},
		8: {makeParams(1, 0, -10, 2, 0, valEd25519), false},
		// test evidence params
		9:  {makeParams(1, 0, 10, 0, 0, valEd25519), false},
		10: {makeParams(1, 0, 10, 2, 2, valEd25519), false},
		11: {makeParams(1000, 0, 10, 2, 1, valEd25519), true},
		12: {makeParams(1, 0, 10, -1, 0, valEd25519), false},
		// test no pubkey type provided
		13: {makeParams(1, 0, 10, 2, 0, []string{}), false},
		// test invalid pubkey type provided
		14: {makeParams(1, 0, 10, 2, 0, []string{"potatoes make good pubkeys"}), false},
	}
	for i, tc := range testCases {
		if tc.valid {
			assert.NoErrorf(t, ValidateConsensusParams(tc.params), "expected no error for valid params (#%d)", i)
		} else {
			assert.Errorf(t, ValidateConsensusParams(tc.params), "expected error for non valid params (#%d)", i)
		}
	}
}

func makeParams(
	blockBytes, blockGas int64,
	blockTimeIotaMs int64,
	evidenceAge int64,
	maxEvidenceBytes int64,
	pubkeyTypes []string,
) tmproto.ConsensusParams {
	return tmproto.ConsensusParams{
		Block: tmproto.BlockParams{
			MaxBytes:   blockBytes,
			MaxGas:     blockGas,
			TimeIotaMs: blockTimeIotaMs,
		},
		Evidence: tmproto.EvidenceParams{
			MaxAgeNumBlocks: evidenceAge,
			MaxAgeDuration:  time.Duration(evidenceAge),
			MaxBytes:        maxEvidenceBytes,
		},
		Validator: tmproto.ValidatorParams{
			PubKeyTypes: pubkeyTypes,
		},
	}
}

func TestConsensusParamsHash(t *testing.T) {
	params := []tmproto.ConsensusParams{
		makeParams(4, 2, 10, 3, 1, valEd25519),
		makeParams(1, 4, 10, 3, 1, valEd25519),
		makeParams(1, 2, 10, 4, 1, valEd25519),
		makeParams(2, 5, 10, 7, 1, valEd25519),
		makeParams(1, 7, 10, 6, 1, valEd25519),
		makeParams(9, 5, 10, 4, 1, valEd25519),
		makeParams(7, 8, 10, 9, 1, valEd25519),
		makeParams(4, 6, 10, 5, 1, valEd25519),
	}

	hashes := make([][]byte, len(params))
	for i := range params {
		hashes[i] = HashConsensusParams(params[i])
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
		params        tmproto.ConsensusParams
		updates       *abci.ConsensusParams
		updatedParams tmproto.ConsensusParams
	}{
		// empty updates
		{
			makeParams(1, 2, 10, 3, 0, valEd25519),
			&abci.ConsensusParams{},
			makeParams(1, 2, 10, 3, 0, valEd25519),
		},
		// fine updates
		{
			makeParams(1, 2, 10, 3, 0, valEd25519),
			&abci.ConsensusParams{
				Block: &abci.BlockParams{
					MaxBytes: 100,
					MaxGas:   200,
				},
				Evidence: &tmproto.EvidenceParams{
					MaxAgeNumBlocks: 300,
					MaxAgeDuration:  time.Duration(300),
					MaxBytes:        50,
				},
				Validator: &tmproto.ValidatorParams{
					PubKeyTypes: valSecp256k1,
				},
			},
			makeParams(100, 200, 10, 300, 50, valSecp256k1),
		},
	}
	for _, tc := range testCases {
		assert.Equal(t, tc.updatedParams, UpdateConsensusParams(tc.params, tc.updates))
	}
}

func TestConsensusParamsUpdate_AppVersion(t *testing.T) {
	params := makeParams(1, 2, 10, 3, 0, valEd25519)

	assert.EqualValues(t, 0, params.Version.AppVersion)

	updated := UpdateConsensusParams(params,
		&abci.ConsensusParams{Version: &tmproto.VersionParams{AppVersion: 1}})

	assert.EqualValues(t, 1, updated.Version.AppVersion)
}
