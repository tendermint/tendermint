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

	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/light/provider"
	provider_mocks "github.com/tendermint/tendermint/light/provider/mocks"
	dbs "github.com/tendermint/tendermint/light/store/db"
	"github.com/tendermint/tendermint/types"
)

const (
	chainID = "test"
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var (
		keys        = genPrivKeys(4)
		vals        = keys.ToValidators(20, 10)
		trustPeriod = 4 * time.Hour

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
		trustOptions = light.TrustOptions{
			Period: 4 * time.Hour,
			Height: 1,
			Hash:   h1.Hash(),
		}
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
	)
	t.Run("ValidateTrustOptions", func(t *testing.T) {
		testCases := []struct {
			err bool
			to  light.TrustOptions
		}{
			{
				false,
				trustOptions,
			},
			{
				true,
				light.TrustOptions{
					Period: -1 * time.Hour,
					Height: 1,
					Hash:   h1.Hash(),
				},
			},
			{
				true,
				light.TrustOptions{
					Period: 1 * time.Hour,
					Height: 0,
					Hash:   h1.Hash(),
				},
			},
			{
				true,
				light.TrustOptions{
					Period: 1 * time.Hour,
					Height: 1,
					Hash:   []byte("incorrect hash"),
				},
			},
		}

		for idx, tc := range testCases {
			t.Run(fmt.Sprint(idx), func(t *testing.T) {
				err := tc.to.ValidateBasic()
				if tc.err {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
	t.Run("SequentialVerification", func(t *testing.T) {
		newKeys := genPrivKeys(4)
		newVals := newKeys.ToValidators(10, 1)
		differentVals, _ := factory.ValidatorSet(ctx, t, 10, 100)

		testCases := []struct {
			name         string
			otherHeaders map[int64]*types.SignedHeader // all except ^
			vals         map[int64]*types.ValidatorSet
			initErr      bool
			verifyErr    bool
		}{
			{
				name:         "good",
				otherHeaders: headerSet,
				vals:         valSet,
				initErr:      false,
				verifyErr:    false,
			},
			{
				"bad: different first header",
				map[int64]*types.SignedHeader{
					// different header
					1: keys.GenSignedHeader(t, chainID, 1, bTime.Add(1*time.Hour), nil, vals, vals,
						hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(keys)),
				},
				map[int64]*types.ValidatorSet{
					1: vals,
				},
				true,
				false,
			},
			{
				"bad: no first signed header",
				map[int64]*types.SignedHeader{},
				map[int64]*types.ValidatorSet{
					1: differentVals,
				},
				true,
				true,
			},
			{
				"bad: different first validator set",
				map[int64]*types.SignedHeader{
					1: h1,
				},
				map[int64]*types.ValidatorSet{
					1: differentVals,
				},
				true,
				true,
			},
			{
				"bad: 1/3 signed interim header",
				map[int64]*types.SignedHeader{
					// trusted header
					1: h1,
					// interim header (1/3 signed)
					2: keys.GenSignedHeader(t, chainID, 2, bTime.Add(1*time.Hour), nil, vals, vals,
						hash("app_hash"), hash("cons_hash"), hash("results_hash"), len(keys)-1, len(keys)),
					// last header (3/3 signed)
					3: keys.GenSignedHeader(t, chainID, 3, bTime.Add(2*time.Hour), nil, vals, vals,
						hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(keys)),
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
					2: keys.GenSignedHeader(t, chainID, 2, bTime.Add(1*time.Hour), nil, vals, vals,
						hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(keys)),
					// last header (1/3 signed)
					3: keys.GenSignedHeader(t, chainID, 3, bTime.Add(2*time.Hour), nil, vals, vals,
						hash("app_hash"), hash("cons_hash"), hash("results_hash"), len(keys)-1, len(keys)),
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
			testCase := tc
			t.Run(testCase.name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				logger := log.NewNopLogger()

				mockNode := mockNodeFromHeadersAndVals(testCase.otherHeaders, testCase.vals)
				mockNode.On("LightBlock", mock.Anything, mock.Anything).Return(nil, provider.ErrLightBlockNotFound)
				c, err := light.NewClient(
					ctx,
					chainID,
					trustOptions,
					mockNode,
					nil,
					dbs.New(dbm.NewMemDB()),
					light.SequentialVerification(),
					light.Logger(logger),
				)

				if testCase.initErr {
					require.Error(t, err)
					return
				}

				require.NoError(t, err)

				_, err = c.VerifyLightBlockAtHeight(ctx, 3, bTime.Add(3*time.Hour))
				if testCase.verifyErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
				mockNode.AssertExpectations(t)
			})
		}

	})
	t.Run("SkippingVerification", func(t *testing.T) {
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
					3: transitKeys.GenSignedHeader(t, chainID, 3, bTime.Add(2*time.Hour), nil, transitVals, transitVals,
						hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(transitKeys)),
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
					2: keys.GenSignedHeader(t, chainID, 2, bTime.Add(1*time.Hour), nil, vals, newVals,
						hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(keys)),
					// last header (0/4 of the original val set signed)
					3: newKeys.GenSignedHeader(t, chainID, 3, bTime.Add(2*time.Hour), nil, newVals, newVals,
						hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(newKeys)),
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
					2: keys.GenSignedHeader(t, chainID, 2, bTime.Add(1*time.Hour), nil, vals, newVals,
						hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, 0),
					// last header (0/4 of the original val set signed)
					3: newKeys.GenSignedHeader(t, chainID, 3, bTime.Add(2*time.Hour), nil, newVals, newVals,
						hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(newKeys)),
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

		bctx, bcancel := context.WithCancel(context.Background())
		defer bcancel()

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(bctx)
				defer cancel()
				logger := log.NewNopLogger()

				mockNode := mockNodeFromHeadersAndVals(tc.otherHeaders, tc.vals)
				mockNode.On("LightBlock", mock.Anything, mock.Anything).Return(nil, provider.ErrLightBlockNotFound)
				c, err := light.NewClient(
					ctx,
					chainID,
					trustOptions,
					mockNode,
					nil,
					dbs.New(dbm.NewMemDB()),
					light.SkippingVerification(light.DefaultTrustLevel),
					light.Logger(logger),
				)
				if tc.initErr {
					require.Error(t, err)
					return
				}

				require.NoError(t, err)

				_, err = c.VerifyLightBlockAtHeight(ctx, 3, bTime.Add(3*time.Hour))
				if tc.verifyErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}

	})
	t.Run("LargeBisectionVerification", func(t *testing.T) {
		// start from a large light block to make sure that the pivot height doesn't select a height outside
		// the appropriate range

		numBlocks := int64(300)
		mockHeaders, mockVals, _ := genLightBlocksWithKeys(t, numBlocks, 101, 2, bTime)

		lastBlock := &types.LightBlock{SignedHeader: mockHeaders[numBlocks], ValidatorSet: mockVals[numBlocks]}
		mockNode := &provider_mocks.Provider{}
		mockNode.On("LightBlock", mock.Anything, numBlocks).
			Return(lastBlock, nil)

		mockNode.On("LightBlock", mock.Anything, int64(200)).
			Return(&types.LightBlock{SignedHeader: mockHeaders[200], ValidatorSet: mockVals[200]}, nil)

		mockNode.On("LightBlock", mock.Anything, int64(256)).
			Return(&types.LightBlock{SignedHeader: mockHeaders[256], ValidatorSet: mockVals[256]}, nil)

		mockNode.On("LightBlock", mock.Anything, int64(0)).Return(lastBlock, nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		trustedLightBlock, err := mockNode.LightBlock(ctx, int64(200))
		require.NoError(t, err)
		c, err := light.NewClient(
			ctx,
			chainID,
			light.TrustOptions{
				Period: 4 * time.Hour,
				Height: trustedLightBlock.Height,
				Hash:   trustedLightBlock.Hash(),
			},
			mockNode,
			nil,
			dbs.New(dbm.NewMemDB()),
			light.SkippingVerification(light.DefaultTrustLevel),
		)
		require.NoError(t, err)
		h, err := c.Update(ctx, bTime.Add(300*time.Minute))
		assert.NoError(t, err)
		height, err := c.LastTrustedHeight()
		require.NoError(t, err)
		require.Equal(t, numBlocks, height)
		h2, err := mockNode.LightBlock(ctx, numBlocks)
		require.NoError(t, err)
		assert.Equal(t, h, h2)
		mockNode.AssertExpectations(t)
	})
	t.Run("BisectionBetweenTrustedHeaders", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockFullNode := mockNodeFromHeadersAndVals(headerSet, valSet)
		c, err := light.NewClient(
			ctx,
			chainID,
			light.TrustOptions{
				Period: 4 * time.Hour,
				Height: 1,
				Hash:   h1.Hash(),
			},
			mockFullNode,
			nil,
			dbs.New(dbm.NewMemDB()),
			light.SkippingVerification(light.DefaultTrustLevel),
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
		c, err := light.NewClient(
			ctx,
			chainID,
			trustOptions,
			mockFullNode,
			nil,
			dbs.New(dbm.NewMemDB()),
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

			c, err := light.NewClient(
				ctx,
				chainID,
				trustOptions,
				mockNode,
				nil,
				trustedStore,
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

			// header1 != h1
			header1 := keys.GenSignedHeader(t, chainID, 1, bTime.Add(1*time.Hour), nil, vals, vals,
				hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(keys))
			mockNode := &provider_mocks.Provider{}

			c, err := light.NewClient(
				ctx,
				chainID,
				light.TrustOptions{
					Period: 4 * time.Hour,
					Height: 1,
					Hash:   header1.Hash(),
				},
				mockNode,
				nil,
				trustedStore,
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

		c, err := light.NewClient(
			ctx,
			chainID,
			trustOptions,
			mockFullNode,
			[]provider.Provider{mockWitnessNode},
			dbs.New(dbm.NewMemDB()),
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
		c, err := light.NewClient(
			ctx,
			chainID,
			trustOptions,
			mockFullNode,
			nil,
			dbs.New(dbm.NewMemDB()),
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
	t.Run("AddProviders", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockFullNode := mockNodeFromHeadersAndVals(map[int64]*types.SignedHeader{
			1: h1,
			2: h2,
		}, valSet)
		logger := log.NewNopLogger()

		c, err := light.NewClient(
			ctx,
			chainID,
			trustOptions,
			mockFullNode,
			nil,
			dbs.New(dbm.NewMemDB()),
			light.Logger(logger),
		)
		require.NoError(t, err)

		closeCh := make(chan struct{})
		go func() {
			// run verification concurrently to make sure it doesn't dead lock
			_, err = c.VerifyLightBlockAtHeight(ctx, 2, bTime.Add(2*time.Hour))
			require.NoError(t, err)
			close(closeCh)
		}()

		c.AddProvider(mockFullNode)
		require.Len(t, c.Witnesses(), 1)
		select {
		case <-closeCh:
		case <-time.After(5 * time.Second):
			t.Fatal("concurent light block verification failed to finish in 5s")
		}
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

		c, err := light.NewClient(
			ctx,
			chainID,
			trustOptions,
			mockDeadNode,
			[]provider.Provider{mockFullNode},
			dbs.New(dbm.NewMemDB()),
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

		c, err := light.NewClient(
			ctx,
			chainID,
			trustOptions,
			mockDeadNode1,
			[]provider.Provider{mockFullNode, mockDeadNode2},
			dbs.New(dbm.NewMemDB()),
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
	t.Run("BackwardsVerification", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		logger := log.NewNopLogger()

		{
			headers, vals, _ := genLightBlocksWithKeys(t, 9, 3, 0, bTime)
			delete(headers, 1)
			delete(headers, 2)
			delete(vals, 1)
			delete(vals, 2)
			mockLargeFullNode := mockNodeFromHeadersAndVals(headers, vals)
			trustHeader, _ := mockLargeFullNode.LightBlock(ctx, 6)

			c, err := light.NewClient(
				ctx,
				chainID,
				light.TrustOptions{
					Period: 4 * time.Minute,
					Height: trustHeader.Height,
					Hash:   trustHeader.Hash(),
				},
				mockLargeFullNode,
				nil,
				dbs.New(dbm.NewMemDB()),
				light.Logger(logger),
			)
			require.NoError(t, err)

			// 1) verify before the trusted header using backwards => expect no error
			h, err := c.VerifyLightBlockAtHeight(ctx, 5, bTime.Add(6*time.Minute))
			require.NoError(t, err)
			if assert.NotNil(t, h) {
				assert.EqualValues(t, 5, h.Height)
			}

			// 2) untrusted header is expired but trusted header is not => expect no error
			h, err = c.VerifyLightBlockAtHeight(ctx, 3, bTime.Add(8*time.Minute))
			assert.NoError(t, err)
			assert.NotNil(t, h)

			// 3) already stored headers should return the header without error
			h, err = c.VerifyLightBlockAtHeight(ctx, 5, bTime.Add(6*time.Minute))
			assert.NoError(t, err)
			assert.NotNil(t, h)

			// 4a) First verify latest header
			_, err = c.VerifyLightBlockAtHeight(ctx, 9, bTime.Add(9*time.Minute))
			require.NoError(t, err)

			// 4b) Verify backwards using bisection => expect no error
			_, err = c.VerifyLightBlockAtHeight(ctx, 7, bTime.Add(9*time.Minute))
			assert.NoError(t, err)
			// shouldn't have verified this header in the process
			_, err = c.TrustedLightBlock(8)
			assert.Error(t, err)

			// 5) Try bisection method, but closest header (at 7) has expired
			// so expect error
			_, err = c.VerifyLightBlockAtHeight(ctx, 8, bTime.Add(12*time.Minute))
			assert.Error(t, err)
			mockLargeFullNode.AssertExpectations(t)

		}
		{
			// 8) provides incorrect hash
			headers := map[int64]*types.SignedHeader{
				2: keys.GenSignedHeader(t, chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
					hash("app_hash2"), hash("cons_hash23"), hash("results_hash30"), 0, len(keys)),
				3: h3,
			}
			vals := valSet
			mockNode := mockNodeFromHeadersAndVals(headers, vals)
			c, err := light.NewClient(
				ctx,
				chainID,
				light.TrustOptions{
					Period: 1 * time.Hour,
					Height: 3,
					Hash:   h3.Hash(),
				},
				mockNode,
				nil,
				dbs.New(dbm.NewMemDB()),
				light.Logger(logger),
			)
			require.NoError(t, err)

			_, err = c.VerifyLightBlockAtHeight(ctx, 2, bTime.Add(1*time.Hour).Add(1*time.Second))
			assert.Error(t, err)
			mockNode.AssertExpectations(t)
		}
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
			trustPeriod,
			mockPrimary,
			[]provider.Provider{mockWitness},
			db,
		)
		require.NoError(t, err)

		// 2) Check light block exists
		h, err := c.TrustedLightBlock(1)
		assert.NoError(t, err)
		assert.EqualValues(t, l1.Height, h.Height)
		mockPrimary.AssertExpectations(t)
		mockWitness.AssertExpectations(t)
	})
	t.Run("RemovesWitnessIfItSendsUsIncorrectHeader", func(t *testing.T) {
		logger := log.NewNopLogger()

		// different headers hash then primary plus less than 1/3 signed (no fork)
		headers1 := map[int64]*types.SignedHeader{
			1: h1,
			2: keys.GenSignedHeaderLastBlockID(t, chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
				hash("app_hash2"), hash("cons_hash"), hash("results_hash"),
				len(keys), len(keys), types.BlockID{Hash: h1.Hash()}),
		}
		vals1 := map[int64]*types.ValidatorSet{
			1: vals,
			2: vals,
		}
		mockBadNode1 := mockNodeFromHeadersAndVals(headers1, vals1)
		mockBadNode1.On("LightBlock", mock.Anything, mock.Anything).Return(nil, provider.ErrLightBlockNotFound)
		mockBadNode1.On("ID").Return("mockBadNode1")

		// header is empty
		headers2 := map[int64]*types.SignedHeader{
			1: h1,
			2: h2,
		}
		vals2 := map[int64]*types.ValidatorSet{
			1: vals,
			2: vals,
		}
		mockBadNode2 := mockNodeFromHeadersAndVals(headers2, vals2)
		mockBadNode2.On("LightBlock", mock.Anything, mock.Anything).Return(nil, provider.ErrLightBlockNotFound)
		mockBadNode2.On("ID").Return("mockBadNode2")

		mockFullNode := mockNodeFromHeadersAndVals(headerSet, valSet)
		mockFullNode.On("ID").Return("mockFullNode")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		lb1, _ := mockBadNode1.LightBlock(ctx, 2)
		require.NotEqual(t, lb1.Hash(), l1.Hash())

		c, err := light.NewClient(
			ctx,
			chainID,
			trustOptions,
			mockFullNode,
			[]provider.Provider{mockBadNode1, mockBadNode2},
			dbs.New(dbm.NewMemDB()),
			light.Logger(logger),
		)
		// witness should have behaved properly -> no error
		require.NoError(t, err)
		assert.EqualValues(t, 2, len(c.Witnesses()))

		// witness behaves incorrectly -> removed from list, no error
		l, err := c.VerifyLightBlockAtHeight(ctx, 2, bTime.Add(2*time.Hour))
		assert.NoError(t, err)
		assert.EqualValues(t, 1, len(c.Witnesses()))
		// light block should still be verified
		assert.EqualValues(t, 2, l.Height)

		// remaining witnesses don't have light block -> error
		_, err = c.VerifyLightBlockAtHeight(ctx, 3, bTime.Add(2*time.Hour))
		if assert.Error(t, err) {
			assert.Equal(t, light.ErrFailedHeaderCrossReferencing, err)
		}
		// witness does not have a light block -> left in the list
		assert.EqualValues(t, 1, len(c.Witnesses()))
		mockBadNode1.AssertExpectations(t)
		mockBadNode2.AssertExpectations(t)
	})
	t.Run("TrustedValidatorSet", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		logger := log.NewNopLogger()

		differentVals, _ := factory.ValidatorSet(ctx, t, 10, 100)
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

		c, err := light.NewClient(
			ctx,
			chainID,
			trustOptions,
			mockFullNode,
			[]provider.Provider{mockBadValSetNode, mockGoodWitness},
			dbs.New(dbm.NewMemDB()),
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

		c, err := light.NewClient(
			ctx,
			chainID,
			trustOptions,
			mockFullNode,
			[]provider.Provider{mockGoodWitness},
			dbs.New(dbm.NewMemDB()),
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

		for i, tc := range testCases {
			testCase := tc
			t.Run(fmt.Sprintf("case: %d", i), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				mockBadNode := mockNodeFromHeadersAndVals(testCase.headers, testCase.vals)
				if testCase.errorToThrow != nil {
					mockBadNode.On("LightBlock", mock.Anything, testCase.errorHeight).Return(nil, testCase.errorToThrow)
				}

				c, err := light.NewClient(
					ctx,
					chainID,
					trustOptions,
					mockBadNode,
					nil,
					dbs.New(dbm.NewMemDB()),
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
