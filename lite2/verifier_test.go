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
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, lastHeight, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	testCases := []struct {
		newHeader      *types.SignedHeader
		newVals        *types.ValidatorSet
		trustingPeriod time.Duration
		now            time.Time
		expErr         error
	}{
		// same header -> no error
		0: {
			header,
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
		},
		// different chainID -> error
		1: {
			keys.GenSignedHeader("different-chainID", nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
		},
		// 3/3 signed -> no error
		2: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
		},
		// 2/3 signed -> no error
		3: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 1, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
		},
		// 1/3 signed -> error
		4: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 3, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
		},
		// vals does not match with what we have -> error
		5: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, keys.ToValidators(10, 1), vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			keys.ToValidators(10, 1),
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
		},
		// vals are inconsistent with newHeader -> error
		6: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			keys.ToValidators(10, 1),
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
		},
		// old header has expired -> error
		7: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			keys.ToValidators(10, 1),
			1 * time.Hour,
			bTime.Add(1 * time.Hour),
			nil,
		},
		// new header is too far into the future -> error
		8: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(4*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			keys.ToValidators(10, 1),
			3 * time.Hour,
			bTime.Add(2 * time.Hour), // not relevant
			nil,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			err := Verify(chainID, header, vals, tc.newHeader, tc.newVals, tc.trustingPeriod, tc.now, 0)

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
