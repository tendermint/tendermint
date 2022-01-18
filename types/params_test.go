package types

import (
	"bytes"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

var (
	valEd25519   = []string{ABCIPubKeyTypeEd25519}
	valSecp256k1 = []string{ABCIPubKeyTypeSecp256k1}
	valSr25519   = []string{ABCIPubKeyTypeSr25519}
)

func TestConsensusParamsValidation(t *testing.T) {
	testCases := []struct {
		params ConsensusParams
		valid  bool
	}{
		// test block params
		{
			params: makeParams(makeParamsArgs{
				blockBytes:   1,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: true,
		},
		{
			params: makeParams(makeParamsArgs{
				blockBytes:   0,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: false,
		},
		{
			params: makeParams(makeParamsArgs{
				blockBytes:   47 * 1024 * 1024,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: true,
		},
		{
			params: makeParams(makeParamsArgs{
				blockBytes:   10,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: true,
		},
		{
			params: makeParams(makeParamsArgs{
				blockBytes:   100 * 1024 * 1024,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: true,
		},
		{
			params: makeParams(makeParamsArgs{
				blockBytes:   101 * 1024 * 1024,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: false,
		},
		{
			params: makeParams(makeParamsArgs{
				blockBytes:   1024 * 1024 * 1024,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: false,
		},
		{
			params: makeParams(makeParamsArgs{
				blockBytes:   1024 * 1024 * 1024,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: false,
		},
		// test evidence params
		{
			params: makeParams(makeParamsArgs{
				blockBytes:       1,
				evidenceAge:      0,
				maxEvidenceBytes: 0,
				precision:        1,
				messageDelay:     1}),
			valid: false,
		},
		{
			params: makeParams(makeParamsArgs{
				blockBytes:       1,
				evidenceAge:      2,
				maxEvidenceBytes: 2,
				precision:        1,
				messageDelay:     1}),
			valid: false,
		},
		{
			params: makeParams(makeParamsArgs{
				blockBytes:       1000,
				evidenceAge:      2,
				maxEvidenceBytes: 1,
				precision:        1,
				messageDelay:     1}),
			valid: true,
		},
		{
			params: makeParams(makeParamsArgs{
				blockBytes:       1,
				evidenceAge:      -1,
				maxEvidenceBytes: 0,
				precision:        1,
				messageDelay:     1}),
			valid: false,
		},
		// test no pubkey type provided
		{
			params: makeParams(makeParamsArgs{
				evidenceAge:  2,
				pubkeyTypes:  []string{},
				precision:    1,
				messageDelay: 1}),
			valid: false,
		},
		// test invalid pubkey type provided
		{
			params: makeParams(makeParamsArgs{
				evidenceAge:  2,
				pubkeyTypes:  []string{"potatoes make good pubkeys"},
				precision:    1,
				messageDelay: 1}),
			valid: false,
		},
		// test invalid pubkey type provided
		{
			params: makeParams(makeParamsArgs{
				evidenceAge:  2,
				precision:    1,
				messageDelay: -1}),
			valid: false,
		},
		{
			params: makeParams(makeParamsArgs{
				evidenceAge:  2,
				precision:    -1,
				messageDelay: 1}),
			valid: false,
		},
	}
	for i, tc := range testCases {
		if tc.valid {
			assert.NoErrorf(t, tc.params.ValidateConsensusParams(), "expected no error for valid params (#%d)", i)
		} else {
			assert.Errorf(t, tc.params.ValidateConsensusParams(), "expected error for non valid params (#%d)", i)
		}
	}
}

type makeParamsArgs struct {
	blockBytes       int64
	blockGas         int64
	evidenceAge      int64
	maxEvidenceBytes int64
	pubkeyTypes      []string
	precision        time.Duration
	messageDelay     time.Duration
}

func makeParams(args makeParamsArgs) ConsensusParams {
	if args.pubkeyTypes == nil {
		args.pubkeyTypes = valEd25519
	}
	return ConsensusParams{
		Block: BlockParams{
			MaxBytes: args.blockBytes,
			MaxGas:   args.blockGas,
		},
		Evidence: EvidenceParams{
			MaxAgeNumBlocks: args.evidenceAge,
			MaxAgeDuration:  time.Duration(args.evidenceAge),
			MaxBytes:        args.maxEvidenceBytes,
		},
		Validator: ValidatorParams{
			PubKeyTypes: args.pubkeyTypes,
		},
		Synchrony: SynchronyParams{
			Precision:    args.precision,
			MessageDelay: args.messageDelay,
		},
	}

}

func TestConsensusParamsHash(t *testing.T) {
	params := []ConsensusParams{
		makeParams(makeParamsArgs{blockBytes: 4, blockGas: 2, evidenceAge: 3, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 1, blockGas: 4, evidenceAge: 3, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 1, blockGas: 2, evidenceAge: 4, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 2, blockGas: 5, evidenceAge: 7, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 1, blockGas: 7, evidenceAge: 6, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 9, blockGas: 5, evidenceAge: 4, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 7, blockGas: 8, evidenceAge: 9, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 4, blockGas: 6, evidenceAge: 5, maxEvidenceBytes: 1}),
	}

	hashes := make([][]byte, len(params))
	for i := range params {
		hashes[i] = params[i].HashConsensusParams()
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
		intialParams  ConsensusParams
		updates       *tmproto.ConsensusParams
		updatedParams ConsensusParams
	}{
		// empty updates
		{
			intialParams:  makeParams(makeParamsArgs{blockBytes: 1, blockGas: 2, evidenceAge: 3}),
			updates:       &tmproto.ConsensusParams{},
			updatedParams: makeParams(makeParamsArgs{blockBytes: 1, blockGas: 2, evidenceAge: 3}),
		},
		{
			// update synchrony params
			intialParams: makeParams(makeParamsArgs{evidenceAge: 3, precision: time.Second, messageDelay: 3 * time.Second}),
			updates: &tmproto.ConsensusParams{
				Synchrony: &tmproto.SynchronyParams{
					Precision:    time.Second * 2,
					MessageDelay: time.Second * 4,
				},
			},
			updatedParams: makeParams(makeParamsArgs{evidenceAge: 3, precision: 2 * time.Second, messageDelay: 4 * time.Second}),
		},
		// fine updates
		{
			intialParams: makeParams(makeParamsArgs{blockBytes: 1, blockGas: 2, evidenceAge: 3}),
			updates: &tmproto.ConsensusParams{
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
			updatedParams: makeParams(makeParamsArgs{
				blockBytes: 100, blockGas: 200,
				evidenceAge:      300,
				maxEvidenceBytes: 50,
				pubkeyTypes:      valSecp256k1}),
		},
		{
			intialParams: makeParams(makeParamsArgs{blockBytes: 1, blockGas: 2, evidenceAge: 3}),
			updates: &tmproto.ConsensusParams{
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
					PubKeyTypes: valSr25519,
				},
			},
			updatedParams: makeParams(makeParamsArgs{
				blockBytes:       100,
				blockGas:         200,
				evidenceAge:      300,
				maxEvidenceBytes: 50,
				pubkeyTypes:      valSr25519}),
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.updatedParams, tc.intialParams.UpdateConsensusParams(tc.updates))
	}
}

func TestConsensusParamsUpdate_AppVersion(t *testing.T) {
	params := makeParams(makeParamsArgs{blockBytes: 1, blockGas: 2, evidenceAge: 3})

	assert.EqualValues(t, 0, params.Version.AppVersion)

	updated := params.UpdateConsensusParams(
		&tmproto.ConsensusParams{Version: &tmproto.VersionParams{AppVersion: 1}})

	assert.EqualValues(t, 1, updated.Version.AppVersion)
}

func TestProto(t *testing.T) {
	params := []ConsensusParams{
		makeParams(makeParamsArgs{blockBytes: 4, blockGas: 2, evidenceAge: 3, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 1, blockGas: 4, evidenceAge: 3, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 1, blockGas: 2, evidenceAge: 4, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 2, blockGas: 5, evidenceAge: 7, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 1, blockGas: 7, evidenceAge: 6, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 9, blockGas: 5, evidenceAge: 4, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 7, blockGas: 8, evidenceAge: 9, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 4, blockGas: 6, evidenceAge: 5, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{precision: time.Second, messageDelay: time.Minute}),
		makeParams(makeParamsArgs{precision: time.Nanosecond, messageDelay: time.Millisecond}),
	}

	for i := range params {
		pbParams := params[i].ToProto()

		oriParams := ConsensusParamsFromProto(pbParams)

		assert.Equal(t, params[i], oriParams)

	}
}
