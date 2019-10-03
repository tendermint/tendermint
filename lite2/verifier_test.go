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
		expErrText     string
	}{
		// same header -> no error
		0: {
			header,
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"expected new header height 1 to be greater than one of old header 1",
		},
		// different chainID -> error
		1: {
			keys.GenSignedHeader("different-chainID", nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"h2.ValidateBasic failed: signedHeader belongs to another chain 'different-chainID' not 'TestVerifyAdjusted'",
		},
		// 3/3 signed -> no error
		2: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"",
		},
		// 2/3 signed -> no error
		3: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 1, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"",
		},
		// 1/3 signed -> error
		4: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 3, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			types.ErrTooMuchChange{Got: 50.0, Needed: 93.00},
			"",
		},
		// vals does not match with what we have -> error
		5: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, keys.ToValidators(10, 1), vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			keys.ToValidators(10, 1),
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"to match those from new header",
		},
		// vals are inconsistent with newHeader -> error
		6: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			keys.ToValidators(10, 1),
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"to match those that were supplied",
		},
		// old header has expired -> error
		7: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			keys.ToValidators(10, 1),
			1 * time.Hour,
			bTime.Add(1 * time.Hour),
			nil,
			"old header has expired",
		},
		// new header is too far into the future -> error
		8: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(4*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			keys.ToValidators(10, 1),
			3 * time.Hour,
			bTime.Add(2 * time.Hour), // not relevant
			ErrNewHeaderTooFarIntoFuture{bTime.Add(4 * time.Hour), bTime.Add(3 * time.Hour)},
			"",
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			tc := tc
			err := Verify(chainID, header, vals, tc.newHeader, tc.newVals, tc.trustingPeriod, tc.now, 0)

			if tc.expErr != nil && assert.Error(t, err) {
				assert.Equal(t, tc.expErr, err)
			} else if tc.expErrText != "" {
				assert.Contains(t, err.Error(), tc.expErrText)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVerifyNonAdjusted(t *testing.T) {

}
