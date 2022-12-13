package types

import (
	"bytes"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
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
		0: {makeParams(1, 0, 2, 0, valEd25519, 0), true},
		1: {makeParams(0, 0, 2, 0, valEd25519, 0), false},
		2: {makeParams(47*1024*1024, 0, 2, 0, valEd25519, 0), true},
		3: {makeParams(10, 0, 2, 0, valEd25519, 0), true},
		4: {makeParams(100*1024*1024, 0, 2, 0, valEd25519, 0), true},
		5: {makeParams(101*1024*1024, 0, 2, 0, valEd25519, 0), false},
		6: {makeParams(1024*1024*1024, 0, 2, 0, valEd25519, 0), false},
		// test evidence params
		7:  {makeParams(1, 0, 0, 0, valEd25519, 0), false},
		8:  {makeParams(1, 0, 2, 2, valEd25519, 0), false},
		9:  {makeParams(1000, 0, 2, 1, valEd25519, 0), true},
		10: {makeParams(1, 0, -1, 0, valEd25519, 0), false},
		// test no pubkey type provided
		11: {makeParams(1, 0, 2, 0, []string{}, 0), false},
		// test invalid pubkey type provided
		12: {makeParams(1, 0, 2, 0, []string{"potatoes make good pubkeys"}, 0), false},
	}
	for i, tc := range testCases {
		if tc.valid {
			assert.NoErrorf(t, tc.params.ValidateBasic(), "expected no error for valid params (#%d)", i)
		} else {
			assert.Errorf(t, tc.params.ValidateBasic(), "expected error for non valid params (#%d)", i)
		}
	}
}

func makeParams(
	blockBytes, blockGas int64,
	evidenceAge int64,
	maxEvidenceBytes int64,
	pubkeyTypes []string,
	abciExtensionHeight int64,
) ConsensusParams {
	return ConsensusParams{
		Block: BlockParams{
			MaxBytes: blockBytes,
			MaxGas:   blockGas,
		},
		Evidence: EvidenceParams{
			MaxAgeNumBlocks: evidenceAge,
			MaxAgeDuration:  time.Duration(evidenceAge),
			MaxBytes:        maxEvidenceBytes,
		},
		Validator: ValidatorParams{
			PubKeyTypes: pubkeyTypes,
		},
		ABCI: ABCIParams{
			VoteExtensionsEnableHeight: abciExtensionHeight,
		},
	}
}

func TestConsensusParamsHash(t *testing.T) {
	params := []ConsensusParams{
		makeParams(4, 2, 3, 1, valEd25519, 0),
		makeParams(1, 4, 3, 1, valEd25519, 0),
		makeParams(1, 2, 4, 1, valEd25519, 0),
		makeParams(2, 5, 7, 1, valEd25519, 0),
		makeParams(1, 7, 6, 1, valEd25519, 0),
		makeParams(9, 5, 4, 1, valEd25519, 0),
		makeParams(7, 8, 9, 1, valEd25519, 0),
		makeParams(4, 6, 5, 1, valEd25519, 0),
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
		updates       *tmproto.ConsensusParams
		updatedParams ConsensusParams
	}{
		// empty updates
		{
			makeParams(1, 2, 3, 0, valEd25519, 0),
			&tmproto.ConsensusParams{},
			makeParams(1, 2, 3, 0, valEd25519, 0),
		},
		// fine updates
		{
			makeParams(1, 2, 3, 0, valEd25519, 0),
			&tmproto.ConsensusParams{
				Block: &tmproto.BlockParams{
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
			makeParams(100, 200, 300, 50, valSecp256k1, 0),
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.updatedParams, tc.params.Update(tc.updates))
	}
}

func TestConsensusParamsUpdate_AppVersion(t *testing.T) {
	params := makeParams(1, 2, 3, 0, valEd25519, 0)

	assert.EqualValues(t, 0, params.Version.App)

	updated := params.Update(
		&tmproto.ConsensusParams{Version: &tmproto.VersionParams{App: 1}})

	assert.EqualValues(t, 1, updated.Version.App)
}

func TestConsensusParamsUpdate_VoteExtensionsEnableHeight(t *testing.T) {
	t.Run("set to height but initial height already run", func(*testing.T) {
		initialParams := makeParams(1, 0, 2, 0, valEd25519, 1)
		update := &tmproto.ConsensusParams{
			Abci: &tmproto.ABCIParams{
				VoteExtensionsEnableHeight: 10,
			},
		}
		require.Error(t, initialParams.ValidateUpdate(update, 1))
		require.Error(t, initialParams.ValidateUpdate(update, 5))
	})
	t.Run("reset to 0", func(t *testing.T) {
		initialParams := makeParams(1, 0, 2, 0, valEd25519, 1)
		update := &tmproto.ConsensusParams{
			Abci: &tmproto.ABCIParams{
				VoteExtensionsEnableHeight: 0,
			},
		}
		require.Error(t, initialParams.ValidateUpdate(update, 1))
	})
	t.Run("set to height before current height run", func(*testing.T) {
		initialParams := makeParams(1, 0, 2, 0, valEd25519, 100)
		update := &tmproto.ConsensusParams{
			Abci: &tmproto.ABCIParams{
				VoteExtensionsEnableHeight: 10,
			},
		}
		require.Error(t, initialParams.ValidateUpdate(update, 11))
		require.Error(t, initialParams.ValidateUpdate(update, 99))
	})
	t.Run("set to height after current height run", func(*testing.T) {
		initialParams := makeParams(1, 0, 2, 0, valEd25519, 300)
		update := &tmproto.ConsensusParams{
			Abci: &tmproto.ABCIParams{
				VoteExtensionsEnableHeight: 99,
			},
		}
		require.NoError(t, initialParams.ValidateUpdate(update, 11))
		require.NoError(t, initialParams.ValidateUpdate(update, 98))
	})
	t.Run("no error when unchanged", func(*testing.T) {
		initialParams := makeParams(1, 0, 2, 0, valEd25519, 100)
		update := &tmproto.ConsensusParams{
			Abci: &tmproto.ABCIParams{
				VoteExtensionsEnableHeight: 100,
			},
		}
		require.NoError(t, initialParams.ValidateUpdate(update, 500))
	})
	t.Run("updated from 0 to 0", func(t *testing.T) {
		initialParams := makeParams(1, 0, 2, 0, valEd25519, 0)
		update := &tmproto.ConsensusParams{
			Abci: &tmproto.ABCIParams{
				VoteExtensionsEnableHeight: 0,
			},
		}
		require.NoError(t, initialParams.ValidateUpdate(update, 100))
	})
}

func TestProto(t *testing.T) {
	params := []ConsensusParams{
		makeParams(4, 2, 3, 1, valEd25519, 1),
		makeParams(1, 4, 3, 1, valEd25519, 1),
		makeParams(1, 2, 4, 1, valEd25519, 1),
		makeParams(2, 5, 7, 1, valEd25519, 1),
		makeParams(1, 7, 6, 1, valEd25519, 1),
		makeParams(9, 5, 4, 1, valEd25519, 1),
		makeParams(7, 8, 9, 1, valEd25519, 1),
		makeParams(4, 6, 5, 1, valEd25519, 1),
	}

	for i := range params {
		pbParams := params[i].ToProto()

		oriParams := ConsensusParamsFromProto(pbParams)

		assert.Equal(t, params[i], oriParams)

	}
}
