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

const (
	chainID = "test"
)

var (
	keys     = genPrivKeys(4)
	vals     = keys.ToValidators(20, 10)
	bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
	h1       = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
		[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	// 3/3 signed
	h2 = keys.GenSignedHeaderLastBlockID(chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
		[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys), types.BlockID{Hash: h1.Hash()})
	// 3/3 signed
	h3 = keys.GenSignedHeaderLastBlockID(chainID, 3, bTime.Add(1*time.Hour), nil, vals, vals,
		[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys), types.BlockID{Hash: h2.Hash()})
	trustPeriod  = 4 * time.Hour
	trustOptions = TrustOptions{
		Period: 4 * time.Hour,
		Height: 1,
		Hash:   h1.Hash(),
	}
	valSet = map[int64]*types.ValidatorSet{
		1: vals,
		2: vals,
		3: vals,
		4: vals,
	}
	headerSet = map[int64]*types.SignedHeader{
		1: h1,
		// interim header (3/3 signed)
		2: h2,
		// last header (3/3 signed)
		3: h3,
	}
	fullNode = mockp.New(
		chainID,
		headerSet,
		valSet,
	)
	deadNode      = mockp.NewDeadMock(chainID)
	largeFullNode = mockp.New(GenMockNode(chainID, 10, 3, 0, bTime))
)

func TestClient_SequentialVerification(t *testing.T) {
	newKeys := genPrivKeys(4)
	newVals := newKeys.ToValidators(10, 1)

	testCases := []struct {
		name         string
		otherHeaders map[int64]*types.SignedHeader // all except ^
		vals         map[int64]*types.ValidatorSet
		initErr      bool
		verifyErr    bool
	}{
		{
			"good",
			headerSet,
			valSet,
			false,
			false,
		},
		{
			"bad: different first header",
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
		{
			"bad: 1/3 signed interim header",
			map[int64]*types.SignedHeader{
				// trusted header
				1: h1,
				// interim header (1/3 signed)
				2: keys.GenSignedHeader(chainID, 2, bTime.Add(1*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), len(keys)-1, len(keys)),
				// last header (3/3 signed)
				3: keys.GenSignedHeader(chainID, 3, bTime.Add(2*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			},
			valSet,
			false,
			true,
		},
		{
			"bad: 1/3 signed last header",
			map[int64]*types.SignedHeader{
				// trusted header
				1: h1,
				// interim header (3/3 signed)
				2: keys.GenSignedHeader(chainID, 2, bTime.Add(1*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
				// last header (1/3 signed)
				3: keys.GenSignedHeader(chainID, 3, bTime.Add(2*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), len(keys)-1, len(keys)),
			},
			valSet,
			false,
			true,
		},
		{
			"bad: different validator set at height 3",
			headerSet,
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: newVals,
			},
			false,
			true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewClient(
				chainID,
				trustOptions,
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
				return
			}

			require.NoError(t, err)

			_, err = c.VerifyHeaderAtHeight(3, bTime.Add(3*time.Hour))
			if tc.verifyErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_SkippingVerification(t *testing.T) {
	// required for 2nd test case
	newKeys := genPrivKeys(4)
	newVals := newKeys.ToValidators(10, 1)

	// 1/3+ of vals, 2/3- of newVals
	transitKeys := keys.Extend(3)
	transitVals := transitKeys.ToValidators(10, 1)

	testCases := []struct {
		name         string
		otherHeaders map[int64]*types.SignedHeader // all except ^
		vals         map[int64]*types.ValidatorSet
		initErr      bool
		verifyErr    bool
	}{
		{
			"good",
			map[int64]*types.SignedHeader{
				// trusted header
				1: h1,
				// last header (3/3 signed)
				3: h3,
			},
			valSet,
			false,
			false,
		},
		{
			"good, but val set changes by 2/3 (1/3 of vals is still present)",
			map[int64]*types.SignedHeader{
				// trusted header
				1: h1,
				3: transitKeys.GenSignedHeader(chainID, 3, bTime.Add(2*time.Hour), nil, transitVals, transitVals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(transitKeys)),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: transitVals,
			},
			false,
			false,
		},
		{
			"good, but val set changes 100% at height 2",
			map[int64]*types.SignedHeader{
				// trusted header
				1: h1,
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
			},
			false,
			false,
		},
		{
			"bad: last header signed by newVals, interim header has no signers",
			map[int64]*types.SignedHeader{
				// trusted header
				1: h1,
				// last header (0/4 of the original val set signed)
				2: keys.GenSignedHeader(chainID, 2, bTime.Add(1*time.Hour), nil, vals, newVals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, 0),
				// last header (0/4 of the original val set signed)
				3: newKeys.GenSignedHeader(chainID, 3, bTime.Add(2*time.Hour), nil, newVals, newVals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(newKeys)),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: newVals,
			},
			false,
			true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewClient(
				chainID,
				trustOptions,
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
				return
			}

			require.NoError(t, err)

			_, err = c.VerifyHeaderAtHeight(3, bTime.Add(3*time.Hour))
			if tc.verifyErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_Cleanup(t *testing.T) {
	c, err := NewClient(
		chainID,
		trustOptions,
		fullNode,
		[]provider.Provider{fullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		Logger(log.TestingLogger()),
	)
	require.NoError(t, err)
	_, err = c.TrustedHeader(1)
	require.NoError(t, err)

	err = c.Cleanup()
	require.NoError(t, err)

	// Check no headers/valsets exist after Cleanup.
	h, err := c.TrustedHeader(1)
	assert.Error(t, err)
	assert.Nil(t, h)

	valSet, _, err := c.TrustedValidatorSet(1)
	assert.Error(t, err)
	assert.Nil(t, valSet)
}

// trustedHeader.Height == options.Height
func TestClientRestoresTrustedHeaderAfterStartup1(t *testing.T) {
	// 1. options.Hash == trustedHeader.Hash
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndValidatorSet(h1, vals)
		require.NoError(t, err)

		c, err := NewClient(
			chainID,
			trustOptions,
			fullNode,
			[]provider.Provider{fullNode},
			trustedStore,
			Logger(log.TestingLogger()),
		)
		require.NoError(t, err)

		h, err := c.TrustedHeader(1)
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, h.Hash(), h1.Hash())

		valSet, _, err := c.TrustedValidatorSet(1)
		assert.NoError(t, err)
		assert.NotNil(t, valSet)
		if assert.NotNil(t, valSet) {
			assert.Equal(t, h.ValidatorsHash.Bytes(), valSet.Hash())
		}
	}

	// 2. options.Hash != trustedHeader.Hash
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndValidatorSet(h1, vals)
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
			valSet,
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

		h, err := c.TrustedHeader(1)
		assert.NoError(t, err)
		if assert.NotNil(t, h) {
			assert.Equal(t, h.Hash(), header1.Hash())
		}

		valSet, _, err := c.TrustedValidatorSet(1)
		assert.NoError(t, err)
		assert.NotNil(t, valSet)
		if assert.NotNil(t, valSet) {
			assert.Equal(t, h.ValidatorsHash.Bytes(), valSet.Hash())
		}
	}
}

// trustedHeader.Height < options.Height
func TestClientRestoresTrustedHeaderAfterStartup2(t *testing.T) {
	// 1. options.Hash == trustedHeader.Hash
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndValidatorSet(h1, vals)
		require.NoError(t, err)

		c, err := NewClient(
			chainID,
			TrustOptions{
				Period: 4 * time.Hour,
				Height: 2,
				Hash:   h2.Hash(),
			},
			fullNode,
			[]provider.Provider{fullNode},
			trustedStore,
			Logger(log.TestingLogger()),
		)
		require.NoError(t, err)

		// Check we still have the 1st header (+header+).
		h, err := c.TrustedHeader(1)
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, h.Hash(), h1.Hash())

		valSet, _, err := c.TrustedValidatorSet(1)
		assert.NoError(t, err)
		assert.NotNil(t, valSet)
		if assert.NotNil(t, valSet) {
			assert.Equal(t, h.ValidatorsHash.Bytes(), valSet.Hash())
		}
	}

	// 2. options.Hash != trustedHeader.Hash
	// This could happen if previous provider was lying to us.
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndValidatorSet(h1, vals)
		require.NoError(t, err)

		// header1 != header
		diffHeader1 := keys.GenSignedHeader(chainID, 1, bTime.Add(1*time.Hour), nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))

		diffHeader2 := keys.GenSignedHeader(chainID, 2, bTime.Add(2*time.Hour), nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))

		primary := mockp.New(
			chainID,
			map[int64]*types.SignedHeader{
				1: diffHeader1,
				2: diffHeader2,
			},
			valSet,
		)

		c, err := NewClient(
			chainID,
			TrustOptions{
				Period: 4 * time.Hour,
				Height: 2,
				Hash:   diffHeader2.Hash(),
			},
			primary,
			[]provider.Provider{primary},
			trustedStore,
			Logger(log.TestingLogger()),
		)
		require.NoError(t, err)

		// Check we no longer have the invalid 1st header (+header+).
		h, err := c.TrustedHeader(1)
		assert.Error(t, err)
		assert.Nil(t, h)

		valSet, _, err := c.TrustedValidatorSet(1)
		assert.Error(t, err)
		assert.Nil(t, valSet)
	}
}

// trustedHeader.Height > options.Height
func TestClientRestoresTrustedHeaderAfterStartup3(t *testing.T) {
	// 1. options.Hash == trustedHeader.Hash
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndValidatorSet(h1, vals)
		require.NoError(t, err)

		//header2 := keys.GenSignedHeader(chainID, 2, bTime.Add(2*time.Hour), nil, vals, vals,
		//	[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
		err = trustedStore.SaveSignedHeaderAndValidatorSet(h2, vals)
		require.NoError(t, err)

		c, err := NewClient(
			chainID,
			trustOptions,
			fullNode,
			[]provider.Provider{fullNode},
			trustedStore,
			Logger(log.TestingLogger()),
		)
		require.NoError(t, err)

		// Check we still have the 1st header (+header+).
		h, err := c.TrustedHeader(1)
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, h.Hash(), h1.Hash())

		valSet, _, err := c.TrustedValidatorSet(1)
		assert.NoError(t, err)
		assert.NotNil(t, valSet)
		if assert.NotNil(t, valSet) {
			assert.Equal(t, h.ValidatorsHash.Bytes(), valSet.Hash())
		}

		// Check we no longer have 2nd header (+header2+).
		h, err = c.TrustedHeader(2)
		assert.Error(t, err)
		assert.Nil(t, h)

		valSet, _, err = c.TrustedValidatorSet(2)
		assert.Error(t, err)
		assert.Nil(t, valSet)
	}

	// 2. options.Hash != trustedHeader.Hash
	// This could happen if previous provider was lying to us.
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndValidatorSet(h1, vals)
		require.NoError(t, err)

		// header1 != header
		header1 := keys.GenSignedHeader(chainID, 1, bTime.Add(1*time.Hour), nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))

		header2 := keys.GenSignedHeader(chainID, 2, bTime.Add(2*time.Hour), nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
		err = trustedStore.SaveSignedHeaderAndValidatorSet(header2, vals)
		require.NoError(t, err)

		primary := mockp.New(
			chainID,
			map[int64]*types.SignedHeader{
				1: header1,
			},
			valSet,
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

		// Check we have swapped invalid 1st header (+header+) with correct one (+header1+).
		h, err := c.TrustedHeader(1)
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, h.Hash(), header1.Hash())

		valSet, _, err := c.TrustedValidatorSet(1)
		assert.NoError(t, err)
		assert.NotNil(t, valSet)
		if assert.NotNil(t, valSet) {
			assert.Equal(t, h.ValidatorsHash.Bytes(), valSet.Hash())
		}

		// Check we no longer have invalid 2nd header (+header2+).
		h, err = c.TrustedHeader(2)
		assert.Error(t, err)
		assert.Nil(t, h)

		valSet, _, err = c.TrustedValidatorSet(2)
		assert.Error(t, err)
		assert.Nil(t, valSet)
	}
}

func TestClient_Update(t *testing.T) {
	c, err := NewClient(
		chainID,
		trustOptions,
		fullNode,
		[]provider.Provider{fullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		Logger(log.TestingLogger()),
	)
	require.NoError(t, err)

	// should result in downloading & verifying header #3
	h, err := c.Update(bTime.Add(2 * time.Hour))
	assert.NoError(t, err)
	if assert.NotNil(t, h) {
		assert.EqualValues(t, 3, h.Height)
	}

	valSet, _, err := c.TrustedValidatorSet(3)
	assert.NoError(t, err)
	if assert.NotNil(t, valSet) {
		assert.Equal(t, h.ValidatorsHash.Bytes(), valSet.Hash())
	}
}

func TestClient_Concurrency(t *testing.T) {
	c, err := NewClient(
		chainID,
		trustOptions,
		fullNode,
		[]provider.Provider{fullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		Logger(log.TestingLogger()),
	)
	require.NoError(t, err)

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

			h, err := c.TrustedHeader(1)
			assert.NoError(t, err)
			assert.NotNil(t, h)

			vals, _, err := c.TrustedValidatorSet(2)
			assert.NoError(t, err)
			assert.NotNil(t, vals)
		}()
	}

	wg.Wait()
}

func TestClientReplacesPrimaryWithWitnessIfPrimaryIsUnavailable(t *testing.T) {
	c, err := NewClient(
		chainID,
		trustOptions,
		deadNode,
		[]provider.Provider{fullNode, fullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		Logger(log.TestingLogger()),
		MaxRetryAttempts(1),
	)

	require.NoError(t, err)
	_, err = c.Update(bTime.Add(2 * time.Hour))
	require.NoError(t, err)

	assert.NotEqual(t, c.Primary(), deadNode)
	assert.Equal(t, 1, len(c.Witnesses()))
}

func TestClient_BackwardsVerification(t *testing.T) {
	{
		trustHeader, _ := largeFullNode.SignedHeader(6)
		c, err := NewClient(
			chainID,
			TrustOptions{
				Period: 4 * time.Minute,
				Height: trustHeader.Height,
				Hash:   trustHeader.Hash(),
			},
			largeFullNode,
			[]provider.Provider{largeFullNode},
			dbs.New(dbm.NewMemDB(), chainID),
			Logger(log.TestingLogger()),
		)
		require.NoError(t, err)

		// 1) verify before the trusted header using backwards => expect no error
		h, err := c.VerifyHeaderAtHeight(5, bTime.Add(6*time.Minute))
		require.NoError(t, err)
		if assert.NotNil(t, h) {
			assert.EqualValues(t, 5, h.Height)
		}

		// 2) untrusted header is expired but trusted header is not => expect no error
		h, err = c.VerifyHeaderAtHeight(3, bTime.Add(8*time.Minute))
		assert.NoError(t, err)
		assert.NotNil(t, h)

		// 3) already stored headers should return the header without error
		h, err = c.VerifyHeaderAtHeight(5, bTime.Add(6*time.Minute))
		assert.NoError(t, err)
		assert.NotNil(t, h)

		// 4a) First verify latest header
		_, err = c.VerifyHeaderAtHeight(9, bTime.Add(9*time.Minute))
		require.NoError(t, err)

		// 4b) Verify backwards using bisection => expect no error
		_, err = c.VerifyHeaderAtHeight(7, bTime.Add(10*time.Minute))
		assert.NoError(t, err)
		// shouldn't have verified this header in the process
		_, err = c.TrustedHeader(8)
		assert.Error(t, err)

		// 5) trusted header has expired => expect error
		_, err = c.VerifyHeaderAtHeight(1, bTime.Add(20*time.Minute))
		assert.Error(t, err)

		// 6) Try bisection method, but closest header (at 7) has expired
		// so change to backwards => expect no error
		_, err = c.VerifyHeaderAtHeight(8, bTime.Add(12*time.Minute))
		assert.NoError(t, err)

	}
	{
		testCases := []struct {
			provider provider.Provider
		}{
			{
				// 7) provides incorrect height
				mockp.New(
					chainID,
					map[int64]*types.SignedHeader{
						1: h1,
						2: keys.GenSignedHeader(chainID, 1, bTime.Add(1*time.Hour), nil, vals, vals,
							[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
						3: h3,
					},
					valSet,
				),
			},
			{
				// 8) provides incorrect hash
				mockp.New(
					chainID,
					map[int64]*types.SignedHeader{
						1: h1,
						2: keys.GenSignedHeader(chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
							[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
						3: h3,
					},
					valSet,
				),
			},
		}

		for _, tc := range testCases {
			c, err := NewClient(
				chainID,
				TrustOptions{
					Period: 1 * time.Hour,
					Height: 3,
					Hash:   h3.Hash(),
				},
				tc.provider,
				[]provider.Provider{tc.provider},
				dbs.New(dbm.NewMemDB(), chainID),
				Logger(log.TestingLogger()),
			)
			require.NoError(t, err)

			_, err = c.VerifyHeaderAtHeight(2, bTime.Add(1*time.Hour).Add(1*time.Second))
			assert.Error(t, err)
		}
	}
}

func TestClient_NewClientFromTrustedStore(t *testing.T) {
	// 1) Initiate DB and fill with a "trusted" header
	db := dbs.New(dbm.NewMemDB(), chainID)
	err := db.SaveSignedHeaderAndValidatorSet(h1, vals)
	require.NoError(t, err)

	c, err := NewClientFromTrustedStore(
		chainID,
		trustPeriod,
		deadNode,
		[]provider.Provider{deadNode},
		db,
	)
	require.NoError(t, err)

	// 2) Check header exists (deadNode is being used to ensure we're not getting
	// it from primary)
	h, err := c.TrustedHeader(1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, h.Height)

	valSet, _, err := c.TrustedValidatorSet(1)
	assert.NoError(t, err)
	assert.NotNil(t, valSet)
	if assert.NotNil(t, valSet) {
		assert.Equal(t, h.ValidatorsHash.Bytes(), valSet.Hash())
	}
}

func TestNewClientErrorsIfAllWitnessesUnavailable(t *testing.T) {
	_, err := NewClient(
		chainID,
		trustOptions,
		fullNode,
		[]provider.Provider{deadNode, deadNode},
		dbs.New(dbm.NewMemDB(), chainID),
		Logger(log.TestingLogger()),
		MaxRetryAttempts(1),
	)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "awaiting response from all witnesses exceeded dropout time")
	}
}

func TestClientRemovesWitnessIfItSendsUsIncorrectHeader(t *testing.T) {
	// different headers hash then primary plus less than 1/3 signed (no fork)
	badProvider1 := mockp.New(
		chainID,
		map[int64]*types.SignedHeader{
			1: h1,
			2: keys.GenSignedHeaderLastBlockID(chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
				[]byte("app_hash2"), []byte("cons_hash"), []byte("results_hash"),
				len(keys), len(keys), types.BlockID{Hash: h1.Hash()}),
		},
		map[int64]*types.ValidatorSet{
			1: vals,
			2: vals,
		},
	)
	// header is empty
	badProvider2 := mockp.New(
		chainID,
		map[int64]*types.SignedHeader{
			1: h1,
			2: h2,
			3: {Header: nil, Commit: nil},
		},
		map[int64]*types.ValidatorSet{
			1: vals,
			2: vals,
		},
	)

	c, err := NewClient(
		chainID,
		trustOptions,
		fullNode,
		[]provider.Provider{badProvider1, badProvider2},
		dbs.New(dbm.NewMemDB(), chainID),
		Logger(log.TestingLogger()),
		MaxRetryAttempts(1),
	)
	// witness should have behaved properly -> no error
	require.NoError(t, err)
	assert.EqualValues(t, 2, len(c.Witnesses()))

	// witness behaves incorrectly -> removed from list, no error
	h, err := c.VerifyHeaderAtHeight(2, bTime.Add(2*time.Hour))
	assert.NoError(t, err)
	assert.EqualValues(t, 1, len(c.Witnesses()))
	// header should still be verified
	assert.EqualValues(t, 2, h.Height)

	// no witnesses left to verify -> error
	_, err = c.VerifyHeaderAtHeight(3, bTime.Add(2*time.Hour))
	assert.Error(t, err)
	assert.EqualValues(t, 0, len(c.Witnesses()))
}

func TestClientTrustedValidatorSet(t *testing.T) {
	c, err := NewClient(
		chainID,
		trustOptions,
		fullNode,
		[]provider.Provider{fullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		Logger(log.TestingLogger()),
	)

	require.NoError(t, err)

	_, err = c.VerifyHeaderAtHeight(2, bTime.Add(2*time.Hour).Add(1*time.Second))
	require.NoError(t, err)

	valSet, height, err := c.TrustedValidatorSet(0)
	assert.NoError(t, err)
	assert.NotNil(t, valSet)
	assert.EqualValues(t, 2, height)
}
