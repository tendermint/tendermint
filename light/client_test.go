package light_test

import (
	"context"
	"sync"
	"testing"
	"time"

	dashcore "github.com/tendermint/tendermint/dashcore/rpc"
	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/light/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light/provider"
	mockp "github.com/tendermint/tendermint/light/provider/mock"
	dbs "github.com/tendermint/tendermint/light/store/db"
	"github.com/tendermint/tendermint/types"
)

const (
	chainID  = "test"
	llmqType = 100
)

var (
	vals, privVals = types.GenerateMockValidatorSet(4)
	keys           = exposeMockPVKeys(privVals, vals.QuorumHash)
	ctx            = context.Background()
	bTime, _       = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
	h1             = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
		hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(keys))
	// 3/3 signed
	h2 = keys.GenSignedHeaderLastBlockID(chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
		hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(keys), types.BlockID{Hash: h1.Hash()})
	// 3/3 signed
	h3 = keys.GenSignedHeaderLastBlockID(chainID, 3, bTime.Add(1*time.Hour), nil, vals, vals,
		hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(keys), types.BlockID{Hash: h2.Hash()})
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
	l1       = &types.LightBlock{SignedHeader: h1, ValidatorSet: vals}
	l2       = &types.LightBlock{SignedHeader: h2, ValidatorSet: vals}
	fullNode = mockp.New(
		chainID,
		headerSet,
		valSet,
		privVals[0],
	)
	deadNode = mockp.NewDeadMock(chainID)
	// largeFullNode = mockp.New(genMockNode(chainID, 10, 3, 0, bTime))

	dashCoreMockClient dashcore.Client
	trustedStore       store.Store
)

func setupDashCoreMockClient(t *testing.T) {
	dashCoreMockClient = dashcore.NewMockClient(chainID, llmqType, privVals[0], true)

	t.Cleanup(func() {
		dashCoreMockClient = nil
	})
}

func setupTrustedStore(t *testing.T) {
	trustedStore = dbs.New(dbm.NewMemDB(), chainID)
	// Adding one block to the store
	err := trustedStore.SaveLightBlock(l1)

	require.NoError(t, err)

	t.Cleanup(func() {
		trustedStore = nil
	})
}

func TestMock(t *testing.T) {
	l, _ := fullNode.LightBlock(ctx, 3)
	assert.Equal(t, int64(3), l.Height)
}

// // start from a large light block to make sure that the pivot height doesn't select a height outside
// // the appropriate range
// func TestClientLargeBisectionVerification(t *testing.T) {
//	veryLargeFullNode := mockp.New(genMockNode(chainID, 100, 3, 0, bTime))
//	trustedLightBlock, err := veryLargeFullNode.LightBlock(ctx, 5)
//	require.NoError(t, err)
//	c, err := light.NewClient(
//		ctx,
//		chainID,
//		light.TrustOptions{
//			Period: 4 * time.Hour,
//			Height: trustedLightBlock.Height,
//			Hash:   trustedLightBlock.Hash(),
//		},
//		veryLargeFullNode,
//		[]provider.Provider{veryLargeFullNode},
//		dbs.New(dbm.NewMemDB(), chainID),
//		light.SkippingVerification(light.DefaultTrustLevel),
//	)
//	require.NoError(t, err)
//	h, err := c.Update(ctx, bTime.Add(100*time.Minute))
//	assert.NoError(t, err)
//	h2, err := veryLargeFullNode.LightBlock(ctx, 100)
//	require.NoError(t, err)
//	assert.Equal(t, h, h2)
// }

func TestClientBisectionBetweenTrustedHeaders(t *testing.T) {
	setupDashCoreMockClient(t)

	c, err := light.NewClient(
		ctx,
		chainID,
		fullNode,
		[]provider.Provider{fullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		dashCoreMockClient,
	)
	require.NoError(t, err)

	_, err = c.VerifyLightBlockAtHeight(ctx, 3, bTime.Add(2*time.Hour))
	require.NoError(t, err)

	// confirm that the client already doesn't have the light block
	_, err = c.TrustedLightBlock(2)
	require.Error(t, err)

	// verify using bisection the light block between the two trusted light blocks
	_, err = c.VerifyLightBlockAtHeight(ctx, 2, bTime.Add(1*time.Hour))
	assert.NoError(t, err)
}

func TestClient_Cleanup(t *testing.T) {
	setupDashCoreMockClient(t)
	setupTrustedStore(t)

	c, err := light.NewClient(
		ctx,
		chainID,
		fullNode,
		[]provider.Provider{fullNode},
		trustedStore,
		dashCoreMockClient,
		light.Logger(log.TestingLogger()),
	)
	require.NoError(t, err)
	_, err = c.TrustedLightBlock(1)
	require.NoError(t, err)

	err = c.Cleanup()
	require.NoError(t, err)

	// Check no light blocks exist after Cleanup.
	l, err := c.TrustedLightBlock(1)
	assert.Error(t, err)
	assert.Nil(t, l)
}

func TestClientRestoresTrustedHeaderAfterStartupWhenHashesAreEqual(t *testing.T) {
	setupDashCoreMockClient(t)

	// options.Hash == trustedHeader.Hash
	trustedStore := dbs.New(dbm.NewMemDB(), chainID)
	err := trustedStore.SaveLightBlock(l1)
	require.NoError(t, err)

	c, err := light.NewClient(
		ctx,
		chainID,
		fullNode,
		[]provider.Provider{fullNode},
		trustedStore,
		dashCoreMockClient,
		light.Logger(log.TestingLogger()),
	)
	require.NoError(t, err)

	l, err := c.TrustedLightBlock(1)
	assert.NoError(t, err)
	assert.NotNil(t, l)
	assert.Equal(t, l.Hash(), h1.Hash())
	assert.Equal(t, l.ValidatorSet.Hash(), h1.ValidatorsHash.Bytes())
}

func TestClientRestoresTrustedHeaderAfterStartup(t *testing.T) {
	setupDashCoreMockClient(t)
	{
		// load the first three headers into the trusted store
		trustedStore := dbs.New(dbm.NewMemDB(), chainID)
		err := trustedStore.SaveLightBlock(l1)
		require.NoError(t, err)

		err = trustedStore.SaveLightBlock(l2)
		require.NoError(t, err)

		c, err := light.NewClient(
			ctx,
			chainID,
			fullNode,
			[]provider.Provider{fullNode},
			trustedStore,
			dashCoreMockClient,
			light.Logger(log.TestingLogger()),
		)
		require.NoError(t, err)

		// Check we still have the 1st light block.
		l, err := c.TrustedLightBlock(1)
		assert.NoError(t, err)
		assert.NotNil(t, l)
		assert.Equal(t, l.Hash(), h1.Hash())
		assert.NoError(t, l.ValidateBasic(chainID))

		l, err = c.TrustedLightBlock(3)
		assert.Error(t, err)
		assert.Nil(t, l)
	}
}

func TestClient_Update(t *testing.T) {
	setupDashCoreMockClient(t)

	c, err := light.NewClientAtHeight(
		ctx,
		2,
		chainID,
		fullNode,
		[]provider.Provider{fullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		dashCoreMockClient,
		light.Logger(log.TestingLogger()),
	)
	require.NoError(t, err)

	// should result in downloading & verifying header #3
	l, err := c.Update(ctx, bTime.Add(2*time.Hour))
	assert.NoError(t, err)
	if assert.NotNil(t, l) {
		assert.EqualValues(t, 3, l.Height)
		assert.NoError(t, l.ValidateBasic(chainID))
	}
}

func TestClient_Concurrency(t *testing.T) {
	setupDashCoreMockClient(t)
	setupTrustedStore(t)

	c, err := light.NewClient(
		ctx,
		chainID,
		fullNode,
		[]provider.Provider{fullNode},
		trustedStore,
		dashCoreMockClient,
		light.Logger(log.TestingLogger()),
	)
	require.NoError(t, err)

	_, err = c.VerifyLightBlockAtHeight(ctx, 2, bTime.Add(2*time.Hour))
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// NOTE: Cleanup, Stop, VerifyLightBlockAtHeight and Verify are not supposed
			// to be concurrently safe.

			assert.Equal(t, chainID, c.ChainID())

			_, err := c.LastTrustedHeight()
			assert.NoError(t, err)

			_, err = c.FirstTrustedHeight()
			assert.NoError(t, err)

			l, err := c.TrustedLightBlock(1)
			assert.NoError(t, err)
			assert.NotNil(t, l)
		}()
	}

	wg.Wait()
}

func TestClientReplacesPrimaryWithWitnessIfPrimaryIsUnavailable(t *testing.T) {
	setupDashCoreMockClient(t)

	c, err := light.NewClient(
		ctx,
		chainID,
		deadNode,
		[]provider.Provider{fullNode, fullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		dashCoreMockClient,
		light.Logger(log.TestingLogger()),
		light.MaxRetryAttempts(1),
	)

	require.NoError(t, err)
	_, err = c.Update(ctx, bTime.Add(2*time.Hour))
	require.NoError(t, err)

	assert.NotEqual(t, c.Primary(), deadNode)
	assert.Equal(t, 1, len(c.Witnesses()))
}

// func TestClient_BackwardsVerification(t *testing.T) {
//	{
//		trustHeader, _ := largeFullNode.LightBlock(ctx, 6)
//		c, err := light.NewClient(
//			ctx,
//			chainID,
//			light.TrustOptions{
//				Period: 4 * time.Minute,
//				Height: trustHeader.Height,
//				Hash:   trustHeader.Hash(),
//			},
//			largeFullNode,
//			[]provider.Provider{largeFullNode},
//			dbs.New(dbm.NewMemDB(), chainID),
//			light.Logger(log.TestingLogger()),
//		)
//		require.NoError(t, err)
//
//		// 1) verify before the trusted header using backwards => expect no error
//		h, err := c.VerifyLightBlockAtHeight(ctx, 5, bTime.Add(6*time.Minute))
//		require.NoError(t, err)
//		if assert.NotNil(t, h) {
//			assert.EqualValues(t, 5, h.Height)
//		}
//
//		// 2) untrusted header is expired but trusted header is not => expect no error
//		h, err = c.VerifyLightBlockAtHeight(ctx, 3, bTime.Add(8*time.Minute))
//		assert.NoError(t, err)
//		assert.NotNil(t, h)
//
//		// 3) already stored headers should return the header without error
//		h, err = c.VerifyLightBlockAtHeight(ctx, 5, bTime.Add(6*time.Minute))
//		assert.NoError(t, err)
//		assert.NotNil(t, h)
//
//		// 4a) First verify latest header
//		_, err = c.VerifyLightBlockAtHeight(ctx, 9, bTime.Add(9*time.Minute))
//		require.NoError(t, err)
//
//		// 4b) Verify backwards using bisection => expect no error
//		_, err = c.VerifyLightBlockAtHeight(ctx, 7, bTime.Add(9*time.Minute))
//		assert.NoError(t, err)
//		// shouldn't have verified this header in the process
//		_, err = c.TrustedLightBlock(8)
//		assert.Error(t, err)
//
//		// 5) Try bisection method, but closest header (at 7) has expired
//		// so expect error
//		_, err = c.VerifyLightBlockAtHeight(ctx, 8, bTime.Add(12*time.Minute))
//		assert.Error(t, err)
//
//	}
//	{
//		testCases := []struct {
//			provider provider.Provider
//		}{
//			{
//				// 7) provides incorrect height
//				mockp.New(
//					chainID,
//					map[int64]*types.SignedHeader{
//						1: h1,
//						2: keys.GenSignedHeader(chainID, 1, bTime.Add(30*time.Minute), nil, vals, vals,
//							hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(keys)),
//						3: h3,
//					},
//					valSet,
//				),
//			},
//			{
//				// 8) provides incorrect hash
//				mockp.New(
//					chainID,
//					map[int64]*types.SignedHeader{
//						1: h1,
//						2: keys.GenSignedHeader(chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
//							hash("app_hash2"), hash("cons_hash23"), hash("results_hash30"), 0, len(keys)),
//						3: h3,
//					},
//					valSet,
//				),
//			},
//		}
//
//		for idx, tc := range testCases {
//			c, err := light.NewClient(
//				ctx,
//				chainID,
//				light.TrustOptions{
//					Period: 1 * time.Hour,
//					Height: 3,
//					Hash:   h3.Hash(),
//				},
//				tc.provider,
//				[]provider.Provider{tc.provider},
//				dbs.New(dbm.NewMemDB(), chainID),
//				light.Logger(log.TestingLogger()),
//			)
//			require.NoError(t, err, idx)
//
//			_, err = c.VerifyLightBlockAtHeight(ctx, 2, bTime.Add(1*time.Hour).Add(1*time.Second))
//			assert.Error(t, err, idx)
//		}
//	}
// }

func TestClient_NewClientFromTrustedStore(t *testing.T) {
	setupDashCoreMockClient(t)

	// 1) Initiate DB and fill with a "trusted" header
	db := dbs.New(dbm.NewMemDB(), chainID)
	err := db.SaveLightBlock(l1)
	require.NoError(t, err)

	c, err := light.NewClientFromTrustedStore(
		chainID,
		deadNode,
		[]provider.Provider{deadNode},
		db,
		dashCoreMockClient,
	)
	require.NoError(t, err)

	// 2) Check light block exists (deadNode is being used to ensure we're not getting
	// it from primary)
	h, err := c.TrustedLightBlock(1)
	assert.NoError(t, err)
	assert.EqualValues(t, l1.Height, h.Height)
}

func TestClient_TrustedValidatorSet(t *testing.T) {
	setupDashCoreMockClient(t)
	differentVals, _ := types.GenerateValidatorSet(10)
	badValSetNode := mockp.New(
		chainID,
		map[int64]*types.SignedHeader{
			1: h1,
			// 3/3 signed, but validator set at height 2 below is invalid -> witness
			// should be removed.
			2: keys.GenSignedHeaderLastBlockID(chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
				hash("app_hash2"), hash("cons_hash"), hash("results_hash"),
				0, len(keys), types.BlockID{Hash: h1.Hash()}),
			3: h3,
		},
		map[int64]*types.ValidatorSet{
			1: vals,
			2: differentVals,
			3: differentVals,
		},
		privVals[0],
	)

	c, err := light.NewClientAtHeight(
		ctx,
		1,
		chainID,
		fullNode,
		[]provider.Provider{badValSetNode, fullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		dashCoreMockClient,
		light.Logger(log.TestingLogger()),
	)
	require.NoError(t, err)
	assert.Equal(t, 2, len(c.Witnesses()))

	_, err = c.VerifyLightBlockAtHeight(ctx, 2, bTime.Add(2*time.Hour).Add(1*time.Second))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(c.Witnesses()))
}

func TestClientPrunesHeadersAndValidatorSets(t *testing.T) {
	setupDashCoreMockClient(t)
	setupTrustedStore(t)

	c, err := light.NewClient(
		ctx,
		chainID,
		fullNode,
		[]provider.Provider{fullNode},
		trustedStore,
		dashCoreMockClient,
		light.Logger(log.TestingLogger()),
		light.PruningSize(1),
	)
	require.NoError(t, err)
	_, err = c.TrustedLightBlock(1)
	require.NoError(t, err)

	h, err := c.Update(ctx, bTime.Add(2*time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(3), h.Height)

	_, err = c.TrustedLightBlock(1)
	assert.Error(t, err)
}

func TestClientEnsureValidHeadersAndValSets(t *testing.T) {
	setupDashCoreMockClient(t)

	emptyValSet := &types.ValidatorSet{
		Validators: nil,
		Proposer:   nil,
	}

	testCases := []struct {
		headers map[int64]*types.SignedHeader
		vals    map[int64]*types.ValidatorSet
		err     bool
	}{
		{
			headerSet,
			valSet,
			false,
		},
		{
			headerSet,
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: nil,
			},
			true,
		},
		{
			map[int64]*types.SignedHeader{
				1: h1,
				2: h2,
				3: nil,
			},
			valSet,
			true,
		},
		{
			headerSet,
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: emptyValSet,
			},
			true,
		},
	}

	for _, tc := range testCases {
		setupTrustedStore(t)
		badNode := mockp.New(
			chainID,
			tc.headers,
			tc.vals,
			privVals[0],
		)
		c, err := light.NewClient(
			ctx,
			chainID,
			badNode,
			[]provider.Provider{badNode, badNode},
			trustedStore,
			dashCoreMockClient,
			light.MaxRetryAttempts(1),
		)
		require.NoError(t, err)

		_, err = c.VerifyLightBlockAtHeight(ctx, 3, bTime.Add(2*time.Hour))
		if tc.err {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}

}
