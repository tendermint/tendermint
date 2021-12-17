package light_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/stretchr/testify/mock"
	"github.com/tendermint/tendermint/internal/test/factory"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dashcore "github.com/tendermint/tendermint/dashcore/rpc"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/light/provider"
	provider_mocks "github.com/tendermint/tendermint/light/provider/mocks"
	"github.com/tendermint/tendermint/light/store"
	dbs "github.com/tendermint/tendermint/light/store/db"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

const (
	chainID  = "test"
	llmqType = 100
)

var (
	vals, privVals = factory.GenerateMockValidatorSet(4)
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
	l1 = &types.LightBlock{SignedHeader: h1, ValidatorSet: vals}
	l2 = &types.LightBlock{SignedHeader: h2, ValidatorSet: vals}
	l3 = &types.LightBlock{SignedHeader: h3, ValidatorSet: vals}

	dashCoreMockClient dashcore.Client
	trustedStore       store.Store
)

func setupDashCoreMockClient(t *testing.T) {
	dashCoreMockClient = dashcore.NewMockClient(chainID, llmqType, privVals[0], true)

	t.Cleanup(func() {
		dashCoreMockClient = nil
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
	mockFullNode := mockNodeFromHeadersAndVals(headerSet, valSet)
	c, err := light.NewClient(
		ctx,
		chainID,
		mockFullNode,
		[]provider.Provider{mockFullNode},
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
}

func TestClient_Cleanup(t *testing.T) {
	setupDashCoreMockClient(t)
	mockFullNode := &provider_mocks.Provider{}
	mockFullNode.On("LightBlock", mock.Anything, int64(1)).Return(l1, nil)

	c, err := light.NewClient(
		ctx,
		chainID,
		mockFullNode,
		[]provider.Provider{mockFullNode},
		dbs.New(dbm.NewMemDB()),
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

func TestClientRestoresTrustedHeaderAfterStartup(t *testing.T) {
	setupDashCoreMockClient(t)
	// 1. options.Hash == trustedHeader.Hash
	t.Run("hashes should match", func(t *testing.T) {
		mockNode := &provider_mocks.Provider{}
		trustedStore := dbs.New(dbm.NewMemDB())
		err := trustedStore.SaveLightBlock(l1)
		require.NoError(t, err)

		c, err := light.NewClient(
			ctx,
			chainID,
			mockNode,
			[]provider.Provider{mockNode},
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
		mockNode.AssertExpectations(t)
	})

	// load the first three headers into the trusted store
	t.Run("hashes should not match", func(t *testing.T) {
		trustedStore := dbs.New(dbm.NewMemDB())
		err := trustedStore.SaveLightBlock(l1)
		require.NoError(t, err)

		err = trustedStore.SaveLightBlock(l2)
		require.NoError(t, err)

		mockNode := &provider_mocks.Provider{}

		c, err := light.NewClient(
			ctx,
			chainID,
			mockNode,
			[]provider.Provider{mockNode},
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
		mockNode.AssertExpectations(t)
	})
}

func TestClient_Update(t *testing.T) {
	setupDashCoreMockClient(t)
	mockFullNode := &provider_mocks.Provider{}
	mockFullNode.On("LightBlock", mock.Anything, int64(0)).Return(l3, nil)
	mockFullNode.On("LightBlock", mock.Anything, int64(1)).Return(l1, nil)
	mockFullNode.On("LightBlock", mock.Anything, int64(3)).Return(l3, nil)

	c, err := light.NewClientAtHeight(
		ctx,
		2,
		chainID,
		mockFullNode,
		[]provider.Provider{mockFullNode},
		dbs.New(dbm.NewMemDB()),
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
	mockNode := &provider_mocks.Provider{}

	c, err := light.NewClient(
		ctx,
		chainID,
		mockNode,
		[]provider.Provider{mockNode},
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
	mockFullNode := &provider_mocks.Provider{}
	mockFullNode.On("LightBlock", mock.Anything, mock.Anything).Return(l1, nil)

	mockDeadNode := &provider_mocks.Provider{}
	mockDeadNode.On("LightBlock", mock.Anything, mock.Anything).Return(nil, provider.ErrNoResponse)
	c, err := light.NewClient(
		ctx,
		chainID,
		mockDeadNode,
		[]provider.Provider{mockDeadNode, mockFullNode},
		dbs.New(dbm.NewMemDB()),
		dashCoreMockClient,
		light.Logger(log.TestingLogger()),
	)

	require.NoError(t, err)
	_, err = c.Update(ctx, bTime.Add(2*time.Hour))
	require.NoError(t, err)

	// the primary should no longer be the deadNode
	assert.NotEqual(t, c.Primary(), mockDeadNode)

	// we should still have the dead node as a witness because it
	// hasn't repeatedly been unresponsive yet
	assert.Equal(t, 2, len(c.Witnesses()))
	mockDeadNode.AssertExpectations(t)
	mockFullNode.AssertExpectations(t)
}

func TestClientReplacesPrimaryWithWitnessIfPrimaryDoesntHaveBlock(t *testing.T) {
	setupDashCoreMockClient(t)
	mockFullNode := &provider_mocks.Provider{}
	mockFullNode.On("LightBlock", mock.Anything, mock.Anything).Return(l1, nil)

	mockDeadNode := &provider_mocks.Provider{}
	mockDeadNode.On("LightBlock", mock.Anything, mock.Anything).Return(nil, provider.ErrLightBlockNotFound)
	c, err := light.NewClient(
		ctx,
		chainID,
		mockDeadNode,
		[]provider.Provider{mockDeadNode, mockFullNode},
		dbs.New(dbm.NewMemDB()),
		dashCoreMockClient,
		light.Logger(log.TestingLogger()),
	)
	require.NoError(t, err)
	_, err = c.Update(ctx, bTime.Add(2*time.Hour))
	require.NoError(t, err)

	// we should still have the dead node as a witness because it
	// hasn't repeatedly been unresponsive yet
	assert.Equal(t, 2, len(c.Witnesses()))
	mockDeadNode.AssertExpectations(t)
	mockFullNode.AssertExpectations(t)
}

func TestClient_NewClientFromTrustedStore(t *testing.T) {
	setupDashCoreMockClient(t)

	// 1) Initiate DB and fill with a "trusted" header
	db := dbs.New(dbm.NewMemDB())
	err := db.SaveLightBlock(l1)
	require.NoError(t, err)
	mockNode := &provider_mocks.Provider{}

	c, err := light.NewClientFromTrustedStore(
		chainID,
		mockNode,
		[]provider.Provider{mockNode},
		db,
		dashCoreMockClient,
	)
	require.NoError(t, err)

	// 2) Check light block exists
	h, err := c.TrustedLightBlock(1)
	assert.NoError(t, err)
	assert.EqualValues(t, l1.Height, h.Height)
	mockNode.AssertExpectations(t)
}

func TestClientRemovesWitnessIfItSendsUsIncorrectHeader(t *testing.T) {
	setupDashCoreMockClient(t)

	// different headers hash then primary plus less than 1/3 signed (no fork)
	headers1 := map[int64]*types.SignedHeader{
		1: h1,
		2: keys.GenSignedHeaderLastBlockID(chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
			hash("app_hash2"), hash("cons_hash"), hash("results_hash"),
			len(keys), len(keys), types.BlockID{Hash: h1.Hash()}),
	}
	vals1 := map[int64]*types.ValidatorSet{
		1: vals,
		2: vals,
	}
	mockBadNode1 := mockNodeFromHeadersAndVals(headers1, vals1)
	mockBadNode1.On("LightBlock", mock.Anything, mock.Anything).Return(nil, provider.ErrLightBlockNotFound)

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

	mockFullNode := mockNodeFromHeadersAndVals(headerSet, valSet)

	lb1, _ := mockBadNode1.LightBlock(ctx, 2)
	require.NotEqual(t, lb1.Hash(), l1.Hash())

	c, err := light.NewClient(
		ctx,
		chainID,
		mockFullNode,
		[]provider.Provider{mockBadNode1, mockBadNode2},
		dbs.New(dbm.NewMemDB()),
		dashCoreMockClient,
		light.Logger(log.TestingLogger()),
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
}

func TestClient_TrustedValidatorSet(t *testing.T) {
	setupDashCoreMockClient(t)
	differentVals, _ := factory.RandValidatorSet(10)
	mockBadValSetNode := mockNodeFromHeadersAndVals(
		map[int64]*types.SignedHeader{
			1: h1,
			// 3/3 signed, but validator set at height 2 below is invalid -> witness
			// should be removed.
			2: keys.GenSignedHeaderLastBlockID(chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
				hash("app_hash2"), hash("cons_hash"), hash("results_hash"),
				0, len(keys), types.BlockID{Hash: h1.Hash()}),
		},
		map[int64]*types.ValidatorSet{
			1: vals,
			2: differentVals,
		})
	mockFullNode := mockNodeFromHeadersAndVals(
		map[int64]*types.SignedHeader{
			1: h1,
			2: h2,
		},
		map[int64]*types.ValidatorSet{
			1: vals,
			2: vals,
		})

	c, err := light.NewClientAtHeight(
		ctx,
		1,
		chainID,
		mockFullNode,
		[]provider.Provider{mockBadValSetNode, mockFullNode},
		dbs.New(dbm.NewMemDB()),
		dashCoreMockClient,
		light.Logger(log.TestingLogger()),
	)
	require.NoError(t, err)
	assert.Equal(t, 2, len(c.Witnesses()))

	_, err = c.VerifyLightBlockAtHeight(ctx, 2, bTime.Add(2*time.Hour).Add(1*time.Second))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(c.Witnesses()))
	mockBadValSetNode.AssertExpectations(t)
	mockFullNode.AssertExpectations(t)
}

func TestClientPrunesHeadersAndValidatorSets(t *testing.T) {
	setupDashCoreMockClient(t)
	mockFullNode := mockNodeFromHeadersAndVals(
		map[int64]*types.SignedHeader{
			1: h1,
			3: h3,
			0: h3,
		},
		map[int64]*types.ValidatorSet{
			1: vals,
			3: vals,
			0: vals,
		})

	c, err := light.NewClient(
		ctx,
		chainID,
		mockFullNode,
		[]provider.Provider{mockFullNode},
		dbs.New(dbm.NewMemDB()),
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
	mockFullNode.AssertExpectations(t)
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
	for tcID, tc := range testCases {
		testCase := tc
		t.Run(fmt.Sprintf("tc_%d", tcID), func(t *testing.T) {
			mockBadNode := mockNodeFromHeadersAndVals(testCase.headers, testCase.vals)
			if testCase.errorToThrow != nil {
				mockBadNode.On("LightBlock", mock.Anything, testCase.errorHeight).Return(nil, testCase.errorToThrow)
			}
			c, err := light.NewClient(
				ctx,
				chainID,
				mockBadNode,
				[]provider.Provider{mockBadNode, mockBadNode},
				dbs.New(dbm.NewMemDB()),
				dashCoreMockClient,
			)
			require.NoError(t, err)

			_, err = c.VerifyLightBlockAtHeight(ctx, 3, bTime.Add(2*time.Hour))
			if tc.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClientHandlesContexts(t *testing.T) {
	mockNode := &provider_mocks.Provider{}
	mockNode.On("LightBlock",
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Err() == nil }),
		int64(1)).Return(l1, nil)
	mockNode.On("LightBlock",
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Err() == context.DeadlineExceeded }),
		mock.Anything).Return(nil, context.DeadlineExceeded)

	mockNode.On("LightBlock",
		mock.MatchedBy(func(ctx context.Context) bool { return ctx.Err() == context.Canceled }),
		mock.Anything).Return(nil, context.Canceled)
	dashCoreMockClient := dashcore.NewMockClient(chainID, llmqType, p.MockPV, true)

	// instantiate the light client with a timeout
	ctxTimeOut, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	_, err := light.NewClient(
		ctxTimeOut,
		chainID,
		p,
		[]provider.Provider{p, p},
		dbs.New(dbm.NewMemDB(), chainID),
		dashCoreMockClient,
	)
	require.Error(t, ctxTimeOut.Err())
	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded))

	// instantiate the client for real
	c, err := light.NewClient(
		ctx,
		chainID,
		p,
		[]provider.Provider{p, p},
		dbs.New(dbm.NewMemDB()),
		dashCoreMockClient,
	)
	require.NoError(t, err)

	// verify a block with a timeout
	ctxTimeOutBlock, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	_, err = c.VerifyLightBlockAtHeight(ctxTimeOutBlock, 101, bTime.Add(100*time.Minute))
	require.Error(t, ctxTimeOutBlock.Err())
	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded))

	// verify a block with a cancel
	ctxCancel, cancel := context.WithCancel(ctx)
	defer cancel()
	time.AfterFunc(10*time.Millisecond, cancel)
	_, err = c.VerifyLightBlockAtHeight(ctxCancel, 101, bTime.Add(100*time.Minute))
	require.Error(t, ctxCancel.Err())
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
}
