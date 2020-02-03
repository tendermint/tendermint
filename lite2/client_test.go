package lite

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/lite2/provider"
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
			[]provider.Provider{mockp.New(
				chainID,
				tc.otherHeaders,
				tc.vals,
			)},
			dbs.New(dbm.NewMemDB(), chainID),
			SequentialVerification(),
		)

		if tc.initErr {
			require.Error(t, err)
			continue
		} else {
			require.NoError(t, err)
		}
		err = c.Start()
		require.NoError(t, err)
		defer c.Stop()

		_, err = c.VerifyHeaderAtHeight(3, bTime.Add(3*time.Hour))
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
			[]provider.Provider{mockp.New(
				chainID,
				tc.otherHeaders,
				tc.vals,
			)},
			dbs.New(dbm.NewMemDB(), chainID),
			SkippingVerification(DefaultTrustLevel),
		)
		if tc.initErr {
			require.Error(t, err)
			continue
		} else {
			require.NoError(t, err)
		}
		err = c.Start()
		require.NoError(t, err)
		defer c.Stop()

		_, err = c.VerifyHeaderAtHeight(3, bTime.Add(3*time.Hour))
		if tc.verifyErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestClientRemovesNoLongerTrustedHeaders(t *testing.T) {
	const (
		chainID = "TestClientRemovesNoLongerTrustedHeaders"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	primary := mockp.New(
		chainID,
		map[int64]*types.SignedHeader{
			// trusted header
			1: header,
			// interim header (3/3 signed)
			2: keys.GenSignedHeader(chainID, 2, bTime.Add(2*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			// last header (3/3 signed)
			3: keys.GenSignedHeader(chainID, 3, bTime.Add(4*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
		},
		map[int64]*types.ValidatorSet{
			1: vals,
			2: vals,
			3: vals,
			4: vals,
		},
	)

	c, err := NewClient(
		chainID,
		TrustOptions{
			Period: 4 * time.Hour,
			Height: 1,
			Hash:   header.Hash(),
		},
		primary,
		[]provider.Provider{primary},
		dbs.New(dbm.NewMemDB(), chainID),
		Logger(log.TestingLogger()),
	)
	require.NoError(t, err)
	err = c.Start()
	require.NoError(t, err)
	defer c.Stop()

	// Verify new headers.
	_, err = c.VerifyHeaderAtHeight(2, bTime.Add(2*time.Hour).Add(1*time.Second))
	require.NoError(t, err)
	now := bTime.Add(4 * time.Hour).Add(1 * time.Second)
	_, err = c.VerifyHeaderAtHeight(3, now)
	require.NoError(t, err)

	// Remove expired headers.
	c.RemoveNoLongerTrustedHeaders(now)

	// Check expired headers are no longer available.
	h, err := c.TrustedHeader(1, now)
	assert.Error(t, err)
	assert.Nil(t, h)

	// Check not expired headers are available.
	h, err = c.TrustedHeader(2, now)
	assert.NoError(t, err)
	assert.NotNil(t, h)
}

func TestClient_Cleanup(t *testing.T) {
	const (
		chainID = "TestClient_Cleanup"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	primary := mockp.New(
		chainID,
		map[int64]*types.SignedHeader{
			// trusted header
			1: header,
		},
		map[int64]*types.ValidatorSet{
			1: vals,
			2: vals,
		},
	)

	c, err := NewClient(
		chainID,
		TrustOptions{
			Period: 4 * time.Hour,
			Height: 1,
			Hash:   header.Hash(),
		},
		primary,
		[]provider.Provider{primary},
		dbs.New(dbm.NewMemDB(), chainID),
		Logger(log.TestingLogger()),
	)
	require.NoError(t, err)
	err = c.Start()
	require.NoError(t, err)

	c.Stop()
	c.Cleanup()

	// Check no headers exist after Cleanup.
	h, err := c.TrustedHeader(1, bTime.Add(1*time.Second))
	assert.Error(t, err)
	assert.Nil(t, h)
}

// trustedHeader.Height == options.Height
func TestClientRestoreTrustedHeaderAfterStartup1(t *testing.T) {
	const (
		chainID = "TestClientRestoreTrustedHeaderAfterStartup1"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	// 1. options.Hash == trustedHeader.Hash
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndNextValidatorSet(header, vals)
		require.NoError(t, err)

		primary := mockp.New(
			chainID,
			map[int64]*types.SignedHeader{
				// trusted header
				1: header,
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
			},
		)

		c, err := NewClient(
			chainID,
			TrustOptions{
				Period: 4 * time.Hour,
				Height: 1,
				Hash:   header.Hash(),
			},
			primary,
			[]provider.Provider{primary},
			trustedStore,
			Logger(log.TestingLogger()),
		)
		require.NoError(t, err)
		err = c.Start()
		require.NoError(t, err)
		defer c.Stop()

		h, err := c.TrustedHeader(1, bTime.Add(1*time.Second))
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, h.Hash(), header.Hash())
	}

	// 2. options.Hash != trustedHeader.Hash
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndNextValidatorSet(header, vals)
		require.NoError(t, err)

		// header1 != header
		header1 := keys.GenSignedHeader(chainID, 1, bTime.Add(1*time.Hour), nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))

		primary := mockp.New(
			chainID,
			map[int64]*types.SignedHeader{
				// trusted header
				1: header1,
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
			},
		)

		c, err := NewClient(
			chainID,
			TrustOptions{
				Period: 4 * time.Hour,
				Height: 1,
				Hash:   header1.Hash(),
			},
			primary,
			[]provider.Provider{primary},
			trustedStore,
			Logger(log.TestingLogger()),
		)
		require.NoError(t, err)
		err = c.Start()
		require.NoError(t, err)
		defer c.Stop()

		h, err := c.TrustedHeader(1, bTime.Add(1*time.Second))
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, h.Hash(), header1.Hash())
	}
}

// trustedHeader.Height < options.Height
func TestClientRestoreTrustedHeaderAfterStartup2(t *testing.T) {
	const (
		chainID = "TestClientRestoreTrustedHeaderAfterStartup2"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	// 1. options.Hash == trustedHeader.Hash
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndNextValidatorSet(header, vals)
		require.NoError(t, err)

		header2 := keys.GenSignedHeader(chainID, 2, bTime.Add(2*time.Hour), nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))

		primary := mockp.New(
			chainID,
			map[int64]*types.SignedHeader{
				1: header,
				2: header2,
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: vals,
			},
		)
		c, err := NewClient(
			chainID,
			TrustOptions{
				Period: 4 * time.Hour,
				Height: 2,
				Hash:   header2.Hash(),
			},
			primary,
			[]provider.Provider{primary},
			trustedStore,
			Logger(log.TestingLogger()),
		)
		require.NoError(t, err)
		err = c.Start()
		require.NoError(t, err)
		defer c.Stop()

		// Check we still have the 1st header (+header+).
		h, err := c.TrustedHeader(1, bTime.Add(2*time.Hour).Add(1*time.Second))
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, h.Hash(), header.Hash())
	}

	// 2. options.Hash != trustedHeader.Hash
	// This could happen if previous provider was lying to us.
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndNextValidatorSet(header, vals)
		require.NoError(t, err)

		// header1 != header
		header1 := keys.GenSignedHeader(chainID, 1, bTime.Add(1*time.Hour), nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))

		header2 := keys.GenSignedHeader(chainID, 2, bTime.Add(2*time.Hour), nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))

		primary := mockp.New(
			chainID,
			map[int64]*types.SignedHeader{
				1: header1,
				2: header2,
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: vals,
			},
		)

		c, err := NewClient(
			chainID,
			TrustOptions{
				Period: 4 * time.Hour,
				Height: 2,
				Hash:   header2.Hash(),
			},
			primary,
			[]provider.Provider{primary},
			trustedStore,
			Logger(log.TestingLogger()),
		)
		require.NoError(t, err)
		err = c.Start()
		require.NoError(t, err)
		defer c.Stop()

		// Check we no longer have the invalid 1st header (+header+).
		h, err := c.TrustedHeader(1, bTime.Add(2*time.Hour).Add(1*time.Second))
		assert.Error(t, err)
		assert.Nil(t, h)
	}
}

// trustedHeader.Height > options.Height
func TestClientRestoreTrustedHeaderAfterStartup3(t *testing.T) {
	const (
		chainID = "TestClientRestoreTrustedHeaderAfterStartup3"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	// 1. options.Hash == trustedHeader.Hash
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndNextValidatorSet(header, vals)
		require.NoError(t, err)

		header2 := keys.GenSignedHeader(chainID, 2, bTime.Add(2*time.Hour), nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
		err = trustedStore.SaveSignedHeaderAndNextValidatorSet(header2, vals)
		require.NoError(t, err)

		primary := mockp.New(
			chainID,
			map[int64]*types.SignedHeader{
				1: header,
				2: header2,
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: vals,
			},
		)

		c, err := NewClient(
			chainID,
			TrustOptions{
				Period: 4 * time.Hour,
				Height: 1,
				Hash:   header.Hash(),
			},
			primary,
			[]provider.Provider{primary},
			trustedStore,
			Logger(log.TestingLogger()),
		)
		require.NoError(t, err)
		err = c.Start()
		require.NoError(t, err)
		defer c.Stop()

		// Check we still have the 1st header (+header+).
		h, err := c.TrustedHeader(1, bTime.Add(2*time.Hour).Add(1*time.Second))
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, h.Hash(), header.Hash())

		// Check we no longer have 2nd header (+header2+).
		h, err = c.TrustedHeader(2, bTime.Add(2*time.Hour).Add(1*time.Second))
		assert.Error(t, err)
		assert.Nil(t, h)
	}

	// 2. options.Hash != trustedHeader.Hash
	// This could happen if previous provider was lying to us.
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndNextValidatorSet(header, vals)
		require.NoError(t, err)

		// header1 != header
		header1 := keys.GenSignedHeader(chainID, 1, bTime.Add(1*time.Hour), nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))

		header2 := keys.GenSignedHeader(chainID, 2, bTime.Add(2*time.Hour), nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
		err = trustedStore.SaveSignedHeaderAndNextValidatorSet(header2, vals)
		require.NoError(t, err)

		primary := mockp.New(
			chainID,
			map[int64]*types.SignedHeader{
				1: header1,
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
			},
		)

		c, err := NewClient(
			chainID,
			TrustOptions{
				Period: 4 * time.Hour,
				Height: 1,
				Hash:   header1.Hash(),
			},
			primary,
			[]provider.Provider{primary},
			trustedStore,
			Logger(log.TestingLogger()),
		)
		require.NoError(t, err)
		err = c.Start()
		require.NoError(t, err)
		defer c.Stop()

		// Check we have swapped invalid 1st header (+header+) with correct one (+header1+).
		h, err := c.TrustedHeader(1, bTime.Add(2*time.Hour).Add(1*time.Second))
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, h.Hash(), header1.Hash())

		// Check we no longer have invalid 2nd header (+header2+).
		h, err = c.TrustedHeader(2, bTime.Add(2*time.Hour).Add(1*time.Second))
		assert.Error(t, err)
		assert.Nil(t, h)
	}
}

func TestClient_Update(t *testing.T) {
	const (
		chainID = "TestClient_Update"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	primary := mockp.New(
		chainID,
		map[int64]*types.SignedHeader{
			// trusted header
			1: header,
			// interim header (3/3 signed)
			2: keys.GenSignedHeader(chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			// last header (3/3 signed)
			3: keys.GenSignedHeader(chainID, 3, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
		},
		map[int64]*types.ValidatorSet{
			1: vals,
			2: vals,
			3: vals,
			4: vals,
		},
	)

	c, err := NewClient(
		chainID,
		TrustOptions{
			Period: 4 * time.Hour,
			Height: 1,
			Hash:   header.Hash(),
		},
		primary,
		[]provider.Provider{primary},
		dbs.New(dbm.NewMemDB(), chainID),
		Logger(log.TestingLogger()),
	)
	require.NoError(t, err)
	err = c.Start()
	require.NoError(t, err)
	defer c.Stop()

	// should result in downloading & verifying header #3
	err = c.Update(bTime.Add(2 * time.Hour))
	require.NoError(t, err)

	h, err := c.TrustedHeader(3, bTime.Add(2*time.Hour))
	assert.NoError(t, err)
	require.NotNil(t, h)
	assert.EqualValues(t, 3, h.Height)
}

func TestClient_Concurrency(t *testing.T) {
	const (
		chainID = "TestClient_Concurrency"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	primary := mockp.New(
		chainID,
		map[int64]*types.SignedHeader{
			// trusted header
			1: header,
			// interim header (3/3 signed)
			2: keys.GenSignedHeader(chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			// last header (3/3 signed)
			3: keys.GenSignedHeader(chainID, 3, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
		},
		map[int64]*types.ValidatorSet{
			1: vals,
			2: vals,
			3: vals,
			4: vals,
		},
	)

	c, err := NewClient(
		chainID,
		TrustOptions{
			Period: 4 * time.Hour,
			Height: 1,
			Hash:   header.Hash(),
		},
		primary,
		[]provider.Provider{primary},
		dbs.New(dbm.NewMemDB(), chainID),
		UpdatePeriod(0),
		Logger(log.TestingLogger()),
	)
	require.NoError(t, err)
	err = c.Start()
	require.NoError(t, err)
	defer c.Stop()

	_, err = c.VerifyHeaderAtHeight(2, bTime.Add(2*time.Hour))
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// NOTE: Cleanup, Stop, VerifyHeaderAtHeight and Verify are not supposed
			// to be concurrenly safe.

			assert.Equal(t, chainID, c.ChainID())

			_, err := c.LastTrustedHeight()
			assert.NoError(t, err)

			_, err = c.FirstTrustedHeight()
			assert.NoError(t, err)

			h, err := c.TrustedHeader(1, bTime.Add(2*time.Hour))
			assert.NoError(t, err)
			assert.NotNil(t, h)

			vals, err := c.TrustedValidatorSet(2, bTime.Add(2*time.Hour))
			assert.NoError(t, err)
			assert.NotNil(t, vals)
		}()
	}

	wg.Wait()
}

func TestProvider_Replacement(t *testing.T) {
	const (
		chainID = "TestProvider_Replacement"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
		primary = mockp.NewDeadMock(chainID)
		witness = mockp.New(
			chainID,
			map[int64]*types.SignedHeader{
				// trusted header
				1: header,
				// interim header (3/3 signed)
				2: keys.GenSignedHeader(chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
				// last header (3/3 signed)
				3: keys.GenSignedHeader(chainID, 3, bTime.Add(1*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: vals,
				4: vals,
			},
		)
	)

	c, err := NewClient(
		chainID,
		TrustOptions{
			Period: 4 * time.Hour,
			Height: 1,
			Hash:   header.Hash(),
		},
		primary,
		[]provider.Provider{witness},
		dbs.New(dbm.NewMemDB(), chainID),
		UpdatePeriod(0),
		Logger(log.TestingLogger()),
	)
	require.NoError(t, err)
	err = c.Start()
	require.NoError(t, err)
	defer c.Stop()
	assert.NotEqual(t, c.Primary(), primary)
	assert.Equal(t, 0, len(c.Witnesses()))

}
