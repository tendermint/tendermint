package light_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	dashcore "github.com/tendermint/tendermint/dash/core"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/light/provider"
	provider_mocks "github.com/tendermint/tendermint/light/provider/mocks"
	dbs "github.com/tendermint/tendermint/light/store/db"
	"github.com/tendermint/tendermint/types"
)

const (
	chainID  = "test"
	llmqType = 100
)

var bTime time.Time

func init() {
	var err error
	bTime, err = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
	if err != nil {
		panic(err)
	}
}

func TestClient(t *testing.T) {
	var (
		vals, privVals = types.RandValidatorSet(4)
		keys           = exposeMockPVKeys(privVals, vals.QuorumHash)

		valSet = map[int64]*types.ValidatorSet{
			1: vals,
			2: vals,
			3: vals,
			4: vals,
		}

		h1 = keys.GenSignedHeader(t, chainID, 1, bTime, nil, vals, vals,
			hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(keys))
		// 3/3 signed
		h2 = keys.GenSignedHeaderLastBlockID(t, chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
			hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(keys), types.BlockID{Hash: h1.Hash()})
		// 3/3 signed
		h3 = keys.GenSignedHeaderLastBlockID(t, chainID, 3, bTime.Add(1*time.Hour), nil, vals, vals,
			hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(keys), types.BlockID{Hash: h2.Hash()})

		headerSet = map[int64]*types.SignedHeader{
			1: h1,
			// interim header (3/3 signed)
			2: h2,
			// last header (3/3 signed)
			3: h3,
		}
		l1 = &types.LightBlock{SignedHeader: h1, ValidatorSet: vals}
		l2 = &types.LightBlock{SignedHeader: h2, ValidatorSet: vals}
		l3 = &types.LightBlock{SignedHeader: h3, ValidatorSet: vals}

		dashCoreMockClient = dashcore.NewMockClient(chainID, llmqType, privVals[0], true)
	)

	//t.Run("LargeBisectionVerification", func(t *testing.T) {
	//	// start from a large light block to make sure that the pivot height doesn't select a height outside
	//	// the appropriate range
	//
	//	numBlocks := int64(300)
	//	mockHeaders, mockVals, _ := genLightBlocksWithKeys(t, numBlocks, 101, 2, bTime)
	//
	//	lastBlock := &types.LightBlock{SignedHeader: mockHeaders[numBlocks], ValidatorSet: mockVals[numBlocks]}
	//	mockNode := &provider_mocks.Provider{}
	//	mockNode.On("LightBlock", mock.Anything, numBlocks).
	//		Return(lastBlock, nil)
	//
	//	mockNode.On("LightBlock", mock.Anything, int64(200)).
	//		Return(&types.LightBlock{SignedHeader: mockHeaders[200], ValidatorSet: mockVals[200]}, nil)
	//
	//	mockNode.On("LightBlock", mock.Anything, int64(256)).
	//		Return(&types.LightBlock{SignedHeader: mockHeaders[256], ValidatorSet: mockVals[256]}, nil)
	//
	//	mockNode.On("LightBlock", mock.Anything, int64(0)).Return(lastBlock, nil)
	//
	//	ctx, cancel := context.WithCancel(context.Background())
	//	defer cancel()
	//
	//	trustedLightBlock, err := mockNode.LightBlock(ctx, int64(200))
	//	require.NoError(t, err)
	//	c, err := light.NewClient(
	//		ctx,
	//		chainID,
	//		light.TrustOptions{
	//			Period: 4 * time.Hour,
	//			Height: trustedLightBlock.Height,
	//			Hash:   trustedLightBlock.Hash(),
	//		},
	//		mockNode,
	//		nil,
	//		dbs.New(dbm.NewMemDB()),
	//		light.SkippingVerification(light.DefaultTrustLevel),
	//	)
	//	require.NoError(t, err)
	//	h, err := c.Update(ctx, bTime.Add(300*time.Minute))
	//	assert.NoError(t, err)
	//	height, err := c.LastTrustedHeight()
	//	require.NoError(t, err)
	//	require.Equal(t, numBlocks, height)
	//	h2, err := mockNode.LightBlock(ctx, numBlocks)
	//	require.NoError(t, err)
	//	assert.Equal(t, h, h2)
	//	mockNode.AssertExpectations(t)
	//})
	t.Run("BisectionBetweenTrustedHeaders", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockFullNode := mockNodeFromHeadersAndVals(headerSet, valSet)
		c, err := light.NewClientAtHeight(
			ctx,
			1,
			chainID,
			mockFullNode,
			nil,
			dbs.New(dbm.NewMemDB()),
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
		mockFullNode.AssertExpectations(t)
	})
	t.Run("Cleanup", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		logger := log.NewNopLogger()

		mockFullNode := &provider_mocks.Provider{}
		mockFullNode.On("LightBlock", mock.Anything, int64(1)).Return(l1, nil)
		c, err := light.NewClientAtHeight(
			ctx,
			1,
			chainID,
			mockFullNode,
			nil,
			dbs.New(dbm.NewMemDB()),
			dashCoreMockClient,
			light.Logger(logger),
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
		mockFullNode.AssertExpectations(t)
	})
	t.Run("RestoresTrustedHeaderAfterStartup", func(t *testing.T) {
		// trustedHeader.Height == options.Height

		bctx, bcancel := context.WithCancel(context.Background())
		defer bcancel()

		// 1. options.Hash == trustedHeader.Hash
		t.Run("hashes should match", func(t *testing.T) {
			ctx, cancel := context.WithCancel(bctx)
			defer cancel()

			logger := log.NewNopLogger()

			mockNode := &provider_mocks.Provider{}
			trustedStore := dbs.New(dbm.NewMemDB())
			err := trustedStore.SaveLightBlock(l1)
			require.NoError(t, err)

			c, err := light.NewClientAtHeight(
				ctx,
				1,
				chainID,
				mockNode,
				nil,
				trustedStore,
				dashCoreMockClient,
				light.Logger(logger),
			)
			require.NoError(t, err)

			l, err := c.TrustedLightBlock(1)
			assert.NoError(t, err)
			assert.NotNil(t, l)
			assert.Equal(t, l.Hash(), h1.Hash())
			assert.Equal(t, l.ValidatorSet.Hash(), h1.ValidatorsHash.Bytes())
			mockNode.AssertExpectations(t)
		})

		// 2. options.Hash != trustedHeader.Hash
		t.Run("hashes should not match", func(t *testing.T) {
			ctx, cancel := context.WithCancel(bctx)
			defer cancel()

			trustedStore := dbs.New(dbm.NewMemDB())
			err := trustedStore.SaveLightBlock(l1)
			require.NoError(t, err)

			logger := log.NewNopLogger()

			mockNode := &provider_mocks.Provider{}

			c, err := light.NewClientAtHeight(
				ctx,
				1,
				chainID,
				mockNode,
				nil,
				trustedStore,
				dashCoreMockClient,
				light.Logger(logger),
			)
			require.NoError(t, err)

			l, err := c.TrustedLightBlock(1)
			assert.NoError(t, err)
			if assert.NotNil(t, l) {
				// client take the trusted store and ignores the trusted options
				assert.Equal(t, l.Hash(), l1.Hash())
				assert.NoError(t, l.ValidateBasic(chainID))
			}

			l, err = c.TrustedLightBlock(3)
			assert.Error(t, err)
			assert.Nil(t, l)

			mockNode.AssertExpectations(t)
		})
	})
	t.Run("Update", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockFullNode := &provider_mocks.Provider{}
		mockFullNode.On("ID").Return("mockFullNode")
		mockFullNode.On("LightBlock", mock.Anything, int64(0)).Return(l3, nil)
		mockFullNode.On("LightBlock", mock.Anything, int64(1)).Return(l1, nil)

		mockWitnessNode := &provider_mocks.Provider{}
		mockWitnessNode.On("ID").Return("mockWitnessNode")
		mockWitnessNode.On("LightBlock", mock.Anything, int64(1)).Return(l1, nil)
		mockWitnessNode.On("LightBlock", mock.Anything, int64(3)).Return(l3, nil)

		logger := log.NewNopLogger()

		c, err := light.NewClientAtHeight(
			ctx,
			1,
			chainID,
			mockFullNode,
			[]provider.Provider{mockWitnessNode},
			dbs.New(dbm.NewMemDB()),
			dashCoreMockClient,
			light.Logger(logger),
		)
		require.NoError(t, err)

		// should result in downloading & verifying header #3
		l, err := c.Update(ctx, bTime.Add(2*time.Hour))
		assert.NoError(t, err)
		if assert.NotNil(t, l) {
			assert.EqualValues(t, 3, l.Height)
			assert.NoError(t, l.ValidateBasic(chainID))
		}
		mockFullNode.AssertExpectations(t)
		mockWitnessNode.AssertExpectations(t)
	})

	t.Run("Concurrency", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		logger := log.NewNopLogger()

		mockFullNode := &provider_mocks.Provider{}
		mockFullNode.On("LightBlock", mock.Anything, int64(2)).Return(l2, nil)
		mockFullNode.On("LightBlock", mock.Anything, int64(1)).Return(l1, nil)
		c, err := light.NewClientAtHeight(
			ctx,
			1,
			chainID,
			mockFullNode,
			nil,
			dbs.New(dbm.NewMemDB()),
			dashCoreMockClient,
			light.Logger(logger),
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
		mockFullNode.AssertExpectations(t)
	})
	t.Run("ReplacesPrimaryWithWitnessIfPrimaryIsUnavailable", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockFullNode := &provider_mocks.Provider{}
		mockFullNode.On("LightBlock", mock.Anything, mock.Anything).Return(l1, nil)
		mockFullNode.On("ID").Return("mockFullNode")
		mockDeadNode := &provider_mocks.Provider{}
		mockDeadNode.On("LightBlock", mock.Anything, mock.Anything).Return(nil, provider.ErrNoResponse)
		mockDeadNode.On("ID").Return("mockDeadNode")

		logger := log.NewNopLogger()

		c, err := light.NewClientAtHeight(
			ctx,
			1,
			chainID,
			mockDeadNode,
			[]provider.Provider{mockFullNode},
			dbs.New(dbm.NewMemDB()),
			dashCoreMockClient,
			light.Logger(logger),
		)

		require.NoError(t, err)
		_, err = c.Update(ctx, bTime.Add(2*time.Hour))
		require.NoError(t, err)

		// the primary should no longer be the deadNode
		assert.NotEqual(t, c.Primary(), mockDeadNode)

		// we should still have the dead node as a witness because it
		// hasn't repeatedly been unresponsive yet
		assert.Equal(t, 1, len(c.Witnesses()))
		mockDeadNode.AssertExpectations(t)
		mockFullNode.AssertExpectations(t)
	})
	t.Run("ReplacesPrimaryWithWitnessIfPrimaryDoesntHaveBlock", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockFullNode := &provider_mocks.Provider{}
		mockFullNode.On("LightBlock", mock.Anything, mock.Anything).Return(l1, nil)
		mockFullNode.On("ID").Return("mockFullNode")

		logger := log.NewNopLogger()

		mockDeadNode1 := &provider_mocks.Provider{}
		mockDeadNode1.On("LightBlock", mock.Anything, mock.Anything).Return(nil, provider.ErrLightBlockNotFound)
		mockDeadNode1.On("ID").Return("mockDeadNode1")

		mockDeadNode2 := &provider_mocks.Provider{}
		mockDeadNode2.On("LightBlock", mock.Anything, mock.Anything).Return(nil, provider.ErrLightBlockNotFound)
		mockDeadNode2.On("ID").Return("mockDeadNode2")

		c, err := light.NewClientAtHeight(
			ctx,
			1,
			chainID,
			mockDeadNode1,
			[]provider.Provider{mockFullNode, mockDeadNode2},
			dbs.New(dbm.NewMemDB()),
			dashCoreMockClient,
			light.Logger(logger),
		)
		require.NoError(t, err)
		_, err = c.Update(ctx, bTime.Add(2*time.Hour))
		require.NoError(t, err)

		// we should still have the dead node as a witness because it
		// hasn't repeatedly been unresponsive yet
		assert.Equal(t, 2, len(c.Witnesses()))
		mockDeadNode1.AssertExpectations(t)
		mockFullNode.AssertExpectations(t)
	})
	t.Run("NewClientFromTrustedStore", func(t *testing.T) {
		// 1) Initiate DB and fill with a "trusted" header
		db := dbs.New(dbm.NewMemDB())
		err := db.SaveLightBlock(l1)
		require.NoError(t, err)
		mockPrimary := &provider_mocks.Provider{}
		mockPrimary.On("ID").Return("mockPrimary")
		mockWitness := &provider_mocks.Provider{}
		mockWitness.On("ID").Return("mockWitness")
		c, err := light.NewClientFromTrustedStore(
			chainID,
			mockPrimary,
			[]provider.Provider{mockWitness},
			db,
			dashCoreMockClient,
		)
		require.NoError(t, err)

		// 2) Check light block exists
		h, err := c.TrustedLightBlock(1)
		assert.NoError(t, err)
		assert.EqualValues(t, l1.Height, h.Height)
		mockPrimary.AssertExpectations(t)
		mockWitness.AssertExpectations(t)
	})
	t.Run("TrustedValidatorSet", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		logger := log.NewNopLogger()

		differentVals, _ := types.RandValidatorSet(10)
		mockBadValSetNode := mockNodeFromHeadersAndVals(
			map[int64]*types.SignedHeader{
				1: h1,
				// 3/3 signed, but validator set at height 2 below is invalid -> witness
				// should be removed.
				2: keys.GenSignedHeaderLastBlockID(t, chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
					hash("app_hash2"), hash("cons_hash"), hash("results_hash"),
					0, len(keys), types.BlockID{Hash: h1.Hash()}),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: differentVals,
			})
		mockBadValSetNode.On("ID").Return("mockBadValSetNode")

		mockFullNode := mockNodeFromHeadersAndVals(
			map[int64]*types.SignedHeader{
				1: h1,
				2: h2,
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
			})
		mockFullNode.On("ID").Return("mockFullNode")

		mockGoodWitness := mockNodeFromHeadersAndVals(
			map[int64]*types.SignedHeader{
				1: h1,
				2: h2,
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
			})
		mockGoodWitness.On("ID").Return("mockGoodWitness")

		c, err := light.NewClientAtHeight(
			ctx,
			1,
			chainID,
			mockFullNode,
			[]provider.Provider{mockBadValSetNode, mockGoodWitness},
			dbs.New(dbm.NewMemDB()),
			dashCoreMockClient,
			light.Logger(logger),
		)
		require.NoError(t, err)
		assert.Equal(t, 2, len(c.Witnesses()))

		_, err = c.VerifyLightBlockAtHeight(ctx, 2, bTime.Add(2*time.Hour).Add(1*time.Second))
		assert.NoError(t, err)
		assert.Equal(t, 1, len(c.Witnesses()))
		mockBadValSetNode.AssertExpectations(t)
		mockFullNode.AssertExpectations(t)
	})
	t.Run("PrunesHeadersAndValidatorSets", func(t *testing.T) {
		mockFullNode := mockNodeFromHeadersAndVals(
			map[int64]*types.SignedHeader{
				1: h1,
				0: h3,
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				0: vals,
			})

		mockFullNode.On("ID").Return("mockFullNode")

		mockGoodWitness := mockNodeFromHeadersAndVals(
			map[int64]*types.SignedHeader{
				1: h1,
				3: h3,
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				3: vals,
			})
		mockGoodWitness.On("ID").Return("mockGoodWitness")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		logger := log.NewNopLogger()

		c, err := light.NewClientAtHeight(
			ctx,
			1,
			chainID,
			mockFullNode,
			[]provider.Provider{mockGoodWitness},
			dbs.New(dbm.NewMemDB()),
			dashCoreMockClient,
			light.Logger(logger),
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
		mockFullNode.AssertExpectations(t)
		mockGoodWitness.AssertExpectations(t)
	})
	t.Run("EnsureValidHeadersAndValSets", func(t *testing.T) {
		emptyValSet := &types.ValidatorSet{
			Validators: nil,
			Proposer:   nil,
		}

		testCases := []struct {
			headers map[int64]*types.SignedHeader
			vals    map[int64]*types.ValidatorSet

			errorToThrow error
			errorHeight  int64

			err bool
		}{
			{
				headers: map[int64]*types.SignedHeader{
					1: h1,
					3: h3,
				},
				vals: map[int64]*types.ValidatorSet{
					1: vals,
					3: vals,
				},
				err: false,
			},
			{
				headers: map[int64]*types.SignedHeader{
					1: h1,
				},
				vals: map[int64]*types.ValidatorSet{
					1: vals,
				},
				errorToThrow: provider.ErrBadLightBlock{Reason: errors.New("nil header or vals")},
				errorHeight:  3,
				err:          true,
			},
			{
				headers: map[int64]*types.SignedHeader{
					1: h1,
				},
				errorToThrow: provider.ErrBadLightBlock{Reason: errors.New("nil header or vals")},
				errorHeight:  3,
				vals:         valSet,
				err:          true,
			},
			{
				headers: map[int64]*types.SignedHeader{
					1: h1,
					3: h3,
				},
				vals: map[int64]*types.ValidatorSet{
					1: vals,
					3: emptyValSet,
				},
				err: true,
			},
		}

		//nolint:scopelint
		for i, tc := range testCases {
			testCase := tc
			t.Run(fmt.Sprintf("case: %d", i), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				mockBadNode := mockNodeFromHeadersAndVals(testCase.headers, testCase.vals)
				if testCase.errorToThrow != nil {
					mockBadNode.On("LightBlock", mock.Anything, testCase.errorHeight).Return(nil, testCase.errorToThrow)
				}

				c, err := light.NewClientAtHeight(
					ctx,
					1,
					chainID,
					mockBadNode,
					nil,
					dbs.New(dbm.NewMemDB()),
					dashCoreMockClient,
				)
				require.NoError(t, err)

				_, err = c.VerifyLightBlockAtHeight(ctx, 3, bTime.Add(2*time.Hour))
				if testCase.err {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
				mockBadNode.AssertExpectations(t)
			})
		}
	})
}
