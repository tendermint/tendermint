package lite

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	mockp "github.com/tendermint/tendermint/lite2/provider/mock"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	"github.com/tendermint/tendermint/types"
)

func TestClient_SequentialVerification(t *testing.T) {
	const (
		chainID = "sequential-verification"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	testCases := []struct {
		otherHeaders map[int64]*types.SignedHeader // all except ^
		vals         map[int64]*types.ValidatorSet
		initErr      bool
		verifyErr    bool
	}{
		// good
		{
			map[int64]*types.SignedHeader{
				// trusted header
				1: header,
				// interim header (3/3 signed)
				2: keys.GenSignedHeader(chainID, 2, bTime.Add(1*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
				// last header (3/3 signed)
				3: keys.GenSignedHeader(chainID, 3, bTime.Add(2*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: vals,
				4: vals,
			},
			false,
			false,
		},
		// bad: different first header
		{
			map[int64]*types.SignedHeader{
				// different header
				1: keys.GenSignedHeader(chainID, 1, bTime.Add(1*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
			},
			true,
			false,
		},
		// bad: 1/3 signed interim header
		{
			map[int64]*types.SignedHeader{
				// trusted header
				1: header,
				// interim header (1/3 signed)
				2: keys.GenSignedHeader(chainID, 2, bTime.Add(1*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), len(keys)-1, len(keys)),
				// last header (3/3 signed)
				3: keys.GenSignedHeader(chainID, 3, bTime.Add(2*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: vals,
				4: vals,
			},
			false,
			true,
		},
		// bad: 1/3 signed last header
		{
			map[int64]*types.SignedHeader{
				// trusted header
				1: header,
				// interim header (3/3 signed)
				2: keys.GenSignedHeader(chainID, 2, bTime.Add(1*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
				// last header (1/3 signed)
				3: keys.GenSignedHeader(chainID, 3, bTime.Add(2*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), len(keys)-1, len(keys)),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: vals,
				4: vals,
			},
			false,
			true,
		},
	}

	for _, tc := range testCases {
		c, err := NewClient(
			chainID,
			TrustOptions{
				Period: 4 * time.Hour,
				Height: 1,
				Hash:   header.Hash(),
			},
			mockp.New(
				chainID,
				tc.otherHeaders,
				tc.vals,
			),
			dbs.New(dbm.NewMemDB(), chainID),
			SequentialVerification(),
		)

		if tc.initErr {
			require.Error(t, err)
			continue
		} else {
			require.NoError(t, err)
		}

		err = c.VerifyHeaderAtHeight(3, bTime.Add(3*time.Hour))
		if tc.verifyErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestClient_SkippingVerification(t *testing.T) {
	const (
		chainID = "skipping-verification"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	// required for 2nd test case
	newKeys := genPrivKeys(4)
	newVals := newKeys.ToValidators(10, 1)

	testCases := []struct {
		otherHeaders map[int64]*types.SignedHeader // all except ^
		vals         map[int64]*types.ValidatorSet
		initErr      bool
		verifyErr    bool
	}{
		// good
		{
			map[int64]*types.SignedHeader{
				// trusted header
				1: header,
				// last header (3/3 signed)
				3: keys.GenSignedHeader(chainID, 3, bTime.Add(2*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: vals,
				4: vals,
			},
			false,
			false,
		},
		// good, val set changes 100% at height 2
		{
			map[int64]*types.SignedHeader{
				// trusted header
				1: header,
				// interim header (3/3 signed)
				2: keys.GenSignedHeader(chainID, 2, bTime.Add(1*time.Hour), nil, vals, newVals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
				// last header (0/4 of the original val set signed)
				3: newKeys.GenSignedHeader(chainID, 3, bTime.Add(2*time.Hour), nil, newVals, newVals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(newKeys)),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: newVals,
				4: newVals,
			},
			false,
			false,
		},
	}

	for _, tc := range testCases {
		c, err := NewClient(
			chainID,
			TrustOptions{
				Period: 4 * time.Hour,
				Height: 1,
				Hash:   header.Hash(),
			},
			mockp.New(
				chainID,
				tc.otherHeaders,
				tc.vals,
			),
			dbs.New(dbm.NewMemDB(), chainID),
			SkippingVerification(DefaultTrustLevel),
		)
		if tc.initErr {
			require.Error(t, err)
			continue
		} else {
			require.NoError(t, err)
		}

		err = c.VerifyHeaderAtHeight(3, bTime.Add(3*time.Hour))
		if tc.verifyErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}
