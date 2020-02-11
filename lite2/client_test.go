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
	h2 = keys.GenSignedHeaderLastBlockID(chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
		[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys), types.BlockID{Hash: h1.Hash()})
	h3 = keys.GenSignedHeaderLastBlockID(chainID, 3, bTime.Add(1*time.Hour), nil, vals, vals,
		[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys), types.BlockID{Hash: h2.Hash()})
	trustPeriod  = 4 * time.Hour
	trustOptions = TrustOptions{
		Period: 4 * time.Hour,
		Height: 1,
		Hash:   h1.Hash(),
	}
	fullNode = mockp.New(
		chainID,
		map[int64]*types.SignedHeader{
			1: h1,
			2: h2,
			3: h3,
		},
		map[int64]*types.ValidatorSet{
			1: vals,
			2: vals,
			3: vals,
			4: vals,
		},
	)
	deadNode = mockp.NewDeadMock(chainID)
)

func TestClient_SequentialVerification(t *testing.T) {

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
				1: h1,
				// interim header (3/3 signed)
				2: h2,
				// last header (3/3 signed)
				3: h3,
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
				1: h1,
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
				1: h1,
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

	// required for 2nd test case
	newKeys := genPrivKeys(4)
	newVals := newKeys.ToValidators(10, 1)

	testCases := []struct {
		otherHeaders map[int64]*types.SignedHeader // all except ^
		vals         map[int64]*types.ValidatorSet
	}{
		// good
		{
			map[int64]*types.SignedHeader{
				// trusted header
				1: h1,
				// last header (3/3 signed)
				3: h3,
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: vals,
				4: vals,
			},
		},
		// good, val set changes 100% at height 2
		{
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
				4: newVals,
			},
		},
	}

	for _, tc := range testCases {
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
		require.NoError(t, err)
		err = c.Start()
		require.NoError(t, err)
		defer c.Stop()

		_, err = c.VerifyHeaderAtHeight(3, bTime.Add(3*time.Hour))
		assert.NoError(t, err)
	}
}

func TestClientRemovesNoLongerTrustedHeaders(t *testing.T) {

	c, err := NewClient(
		chainID,
		trustOptions,
		fullNode,
		[]provider.Provider{fullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		Logger(log.TestingLogger()),
	)

	assert.NotPanics(t, func() {
		now := bTime.Add(4 * time.Hour).Add(1 * time.Second)
		c.RemoveNoLongerTrustedHeaders(now)
	})

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

	c, err := NewClient(
		chainID,
		trustOptions,
		fullNode,
		[]provider.Provider{fullNode},
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

	// 1. options.Hash == trustedHeader.Hash
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndNextValidatorSet(h1, vals)
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
		err = c.Start()
		require.NoError(t, err)
		defer c.Stop()

		h, err := c.TrustedHeader(1, bTime.Add(1*time.Second))
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, h.Hash(), h1.Hash())
	}

	// 2. options.Hash != trustedHeader.Hash
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndNextValidatorSet(h1, vals)
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

	// 1. options.Hash == trustedHeader.Hash
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndNextValidatorSet(h1, vals)
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
		err = c.Start()
		require.NoError(t, err)
		defer c.Stop()

		// Check we still have the 1st header (+header+).
		h, err := c.TrustedHeader(1, bTime.Add(2*time.Hour).Add(1*time.Second))
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, h.Hash(), h1.Hash())
	}

	// 2. options.Hash != trustedHeader.Hash
	// This could happen if previous provider was lying to us.
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndNextValidatorSet(h1, vals)
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
				Hash:   diffHeader2.Hash(),
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

	// 1. options.Hash == trustedHeader.Hash
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndNextValidatorSet(h1, vals)
		require.NoError(t, err)

		//header2 := keys.GenSignedHeader(chainID, 2, bTime.Add(2*time.Hour), nil, vals, vals,
		//	[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
		err = trustedStore.SaveSignedHeaderAndNextValidatorSet(h2, vals)
		require.NoError(t, err)

		primary := mockp.New(
			chainID,
			map[int64]*types.SignedHeader{
				1: h1,
				2: h2,
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: vals,
			},
		)

		c, err := NewClient(
			chainID,
			trustOptions,
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
		assert.Equal(t, h.Hash(), h1.Hash())

		// Check we no longer have 2nd header (+header2+).
		h, err = c.TrustedHeader(2, bTime.Add(2*time.Hour).Add(1*time.Second))
		assert.Error(t, err)
		assert.Nil(t, h)
	}

	// 2. options.Hash != trustedHeader.Hash
	// This could happen if previous provider was lying to us.
	{
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveSignedHeaderAndNextValidatorSet(h1, vals)
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

	c, err := NewClient(
		chainID,
		trustOptions,
		fullNode,
		[]provider.Provider{fullNode},
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

	c, err := NewClient(
		chainID,
		trustOptions,
		fullNode,
		[]provider.Provider{fullNode},
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

	c, err := NewClient(
		chainID,
		trustOptions,
		deadNode,
		[]provider.Provider{fullNode, fullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		UpdatePeriod(0),
		Logger(log.TestingLogger()),
		MaxRetryAttempts(1),
	)
	require.NoError(t, err)
	err = c.Update(bTime.Add(2 * time.Hour))
	require.NoError(t, err)

	assert.NotEqual(t, c.Primary(), deadNode)
	assert.Equal(t, 1, len(c.Witnesses()))
}

func TestProvider_TrustedHeaderFetchesMissingHeader(t *testing.T) {

	c, err := NewClient(
		chainID,
		TrustOptions{
			Period: 1 * time.Hour,
			Height: 3,
			Hash:   h3.Hash(),
		},
		fullNode,
		[]provider.Provider{fullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		UpdatePeriod(0),
		Logger(log.TestingLogger()),
	)
	require.NoError(t, err)
	err = c.Start()
	require.NoError(t, err)
	defer c.Stop()

	// 1) header is missing => expect no error
	h, err := c.TrustedHeader(2, bTime.Add(1*time.Hour).Add(1*time.Second))
	require.NoError(t, err)
	if assert.NotNil(t, h) {
		assert.EqualValues(t, 2, h.Height)
	}

	// 2) header is missing, but it's expired => expect error
	h, err = c.TrustedHeader(1, bTime.Add(1*time.Hour).Add(1*time.Second))
	assert.Error(t, err)
	assert.Nil(t, h)
}

func Test_NewClientFromTrustedStore(t *testing.T) {

	// 1) Initiate DB and fill with a "trusted" header
	db := dbs.New(dbm.NewMemDB(), chainID)
	err := db.SaveSignedHeaderAndNextValidatorSet(h1, vals)
	require.NoError(t, err)

	// 2) Initialize Lite Client from Trusted Store
	c, err := NewClientFromTrustedStore(
		chainID,
		trustPeriod,
		fullNode,
		[]provider.Provider{fullNode},
		db,
	)
	require.NoError(t, err)

	// 3) Check header exists through the lite clients eyes
	h, err := c.TrustedHeader(1, bTime.Add(1*time.Second))
	assert.NoError(t, err)
	assert.EqualValues(t, 1, h.Height)
}

func TestCompareWithWitnesses(t *testing.T) {

	c, err := NewClient(
		chainID,
		trustOptions,
		fullNode,
		[]provider.Provider{deadNode, deadNode, deadNode},
		dbs.New(dbm.NewMemDB(), chainID),
		UpdatePeriod(0),
		Logger(log.TestingLogger()),
		MaxRetryAttempts(1),
	)
	require.NoError(t, err)
	err = c.Update(time.Now())
	assert.Error(t, err)

}
