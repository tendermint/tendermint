package lite

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/types"
)

func TestVerifyAdjustedHeaders(t *testing.T) {
	const (
		chainID    = "TestVerifyAdjusted"
		lastHeight = 1
		nextHeight = 2
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals   = keys.ToValidators(20, 10)
		header = keys.GenSignedHeader(chainID, lastHeight, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	testCases := []struct {
		newHeader *types.SignedHeader
		newVals   *types.ValidatorSet
		expErr    error
	}{
		// same header -> no error
		0: {
			header,
			vals,
			nil,
		},
		// different chainID -> error
		1: {
			keys.GenSignedHeader("different-chainID", nextHeight, nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			vals,
			nil,
		},
		// 3/3 signed -> no error
		2: {
			keys.GenSignedHeader(chainID, nextHeight, nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			vals,
			nil,
		},
		// 2/3 signed -> no error
		3: {
			keys.GenSignedHeader(chainID, nextHeight, nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 1, len(keys)),
			vals,
			nil,
		},
		// 1/3 signed -> error
		4: {
			keys.GenSignedHeader(chainID, nextHeight, nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 3, len(keys)),
			vals,
			nil,
		},
		// vals does not match with what we have -> error
		5: {
			keys.GenSignedHeader(chainID, nextHeight, nil, keys.ToValidators(10, 1), vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			vals,
			nil,
		},
		// vals are inconsistent with newHeader -> error
		6: {
			keys.GenSignedHeader(chainID, nextHeight, nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			keys.ToValidators(10, 1),
			nil,
		},
	}

	now := time.Now()
	trustingPeriod := 1 * time.Hour
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			err := Verify(chainID, header, vals, tc.newHeader, tc.newVals, trustingPeriod, now, 0)

			if tc.expErr == nil {
				assert.NoError(t, err)
			} else {
				if assert.Error(t, err) {
					assert.Equal(t, tc.expErr, err)
				}
			}
		})
	}
}

func TestVerifyNonAdjusted(t *testing.T) {

}
