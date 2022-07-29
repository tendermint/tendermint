package light_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/light/provider"
	provider_mocks "github.com/tendermint/tendermint/light/provider/mocks"
	dbs "github.com/tendermint/tendermint/light/store/db"
	"github.com/tendermint/tendermint/types"
)

func TestLightClientAttackEvidence_Lunatic(t *testing.T) {
	logger := log.NewNopLogger()

	// primary performs a lunatic attack
	var (
		latestHeight      = int64(3)
		valSize           = 5
		divergenceHeight  = int64(2)
		primaryHeaders    = make(map[int64]*types.SignedHeader, latestHeight)
		primaryValidators = make(map[int64]*types.ValidatorSet, latestHeight)
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	witnessHeaders, witnessValidators, chainKeys := genLightBlocksWithKeys(t, latestHeight, valSize, 2, bTime)

	forgedKeys := chainKeys[divergenceHeight-1].ChangeKeys(3) // we change 3 out of the 5 validators (still 2/5 remain)
	forgedVals := forgedKeys.ToValidators(2, 0)

	for height := int64(1); height <= latestHeight; height++ {
		if height < divergenceHeight {
			primaryHeaders[height] = witnessHeaders[height]
			primaryValidators[height] = witnessValidators[height]
			continue
		}
		primaryHeaders[height] = forgedKeys.GenSignedHeader(t, chainID, height, bTime.Add(time.Duration(height)*time.Minute),
			nil, forgedVals, forgedVals, hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(forgedKeys))
		primaryValidators[height] = forgedVals
	}

	// never called, delete it to make mockery asserts pass
	delete(witnessHeaders, 2)
	delete(primaryHeaders, 2)

	mockWitness := mockNodeFromHeadersAndVals(witnessHeaders, witnessValidators)
	mockWitness.On("ID").Return("mockWitness")
	mockWitness.On("ReportEvidence", mock.Anything, mock.MatchedBy(func(evidence types.Evidence) bool {
		evAgainstPrimary := &types.LightClientAttackEvidence{
			// after the divergence height the valset doesn't change so we expect the evidence to be for the latest height
			ConflictingBlock: &types.LightBlock{
				SignedHeader: primaryHeaders[latestHeight],
				ValidatorSet: primaryValidators[latestHeight],
			},
			CommonHeight: 1,
		}
		return bytes.Equal(evidence.Hash(), evAgainstPrimary.Hash())
	})).Return(nil)

	mockPrimary := mockNodeFromHeadersAndVals(primaryHeaders, primaryValidators)
	mockPrimary.On("ID").Return("mockPrimary")
	mockPrimary.On("ReportEvidence", mock.Anything, mock.MatchedBy(func(evidence types.Evidence) bool {
		evAgainstWitness := &types.LightClientAttackEvidence{
			// when forming evidence against witness we learn that the canonical chain continued to change validator sets
			// hence the conflicting block is at 7
			ConflictingBlock: &types.LightBlock{
				SignedHeader: witnessHeaders[divergenceHeight+1],
				ValidatorSet: witnessValidators[divergenceHeight+1],
			},
			CommonHeight: divergenceHeight - 1,
		}
		return bytes.Equal(evidence.Hash(), evAgainstWitness.Hash())
	})).Return(nil)

	c, err := light.NewClient(
		ctx,
		chainID,
		light.TrustOptions{
			Period: 4 * time.Hour,
			Height: 1,
			Hash:   primaryHeaders[1].Hash(),
		},
		mockPrimary,
		[]provider.Provider{mockWitness},
		dbs.New(dbm.NewMemDB()),
		light.Logger(logger),
	)
	require.NoError(t, err)

	// Check verification returns an error.
	_, err = c.VerifyLightBlockAtHeight(ctx, latestHeight, bTime.Add(1*time.Hour))
	if assert.Error(t, err) {
		assert.Equal(t, light.ErrLightClientAttack, err)
	}

	mockWitness.AssertExpectations(t)
	mockPrimary.AssertExpectations(t)
}

func TestLightClientAttackEvidence_Equivocation(t *testing.T) {
	cases := []struct {
		name                      string
		lightOption               light.Option
		unusedWitnessBlockHeights []int64
		unusedPrimaryBlockHeights []int64
		latestHeight              int64
		divergenceHeight          int64
	}{
		{
			name:                      "sequential",
			lightOption:               light.SequentialVerification(),
			unusedWitnessBlockHeights: []int64{4, 6},
			latestHeight:              int64(5),
			divergenceHeight:          int64(3),
		},
		{
			name:                      "skipping",
			lightOption:               light.SkippingVerification(light.DefaultTrustLevel),
			unusedWitnessBlockHeights: []int64{2, 4, 6},
			unusedPrimaryBlockHeights: []int64{2, 4, 6},
			latestHeight:              int64(5),
			divergenceHeight:          int64(3),
		},
	}

	bctx, bcancel := context.WithCancel(context.Background())
	defer bcancel()

	for _, tc := range cases {
		testCase := tc
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(bctx)
			defer cancel()

			logger := log.NewNopLogger()

			// primary performs an equivocation attack
			var (
				valSize        = 5
				primaryHeaders = make(map[int64]*types.SignedHeader, testCase.latestHeight)
				// validators don't change in this network (however we still use a map just for convenience)
				primaryValidators = make(map[int64]*types.ValidatorSet, testCase.latestHeight)
			)
			witnessHeaders, witnessValidators, chainKeys := genLightBlocksWithKeys(t,
				testCase.latestHeight+1, valSize, 2, bTime)
			for height := int64(1); height <= testCase.latestHeight; height++ {
				if height < testCase.divergenceHeight {
					primaryHeaders[height] = witnessHeaders[height]
					primaryValidators[height] = witnessValidators[height]
					continue
				}
				// we don't have a network partition so we will make 4/5 (greater than 2/3) malicious and vote again for
				// a different block (which we do by adding txs)
				primaryHeaders[height] = chainKeys[height].GenSignedHeader(t, chainID, height,
					bTime.Add(time.Duration(height)*time.Minute), []types.Tx{[]byte("abcd")},
					witnessValidators[height], witnessValidators[height+1], hash("app_hash"),
					hash("cons_hash"), hash("results_hash"), 0, len(chainKeys[height])-1)
				primaryValidators[height] = witnessValidators[height]
			}

			for _, height := range testCase.unusedWitnessBlockHeights {
				delete(witnessHeaders, height)
			}
			mockWitness := mockNodeFromHeadersAndVals(witnessHeaders, witnessValidators)
			mockWitness.On("ID").Return("mockWitness")

			for _, height := range testCase.unusedPrimaryBlockHeights {
				delete(primaryHeaders, height)
			}
			mockPrimary := mockNodeFromHeadersAndVals(primaryHeaders, primaryValidators)
			mockPrimary.On("ID").Return("mockPrimary")

			// Check evidence was sent to both full nodes.
			// Common height should be set to the height of the divergent header in the instance
			// of an equivocation attack and the validator sets are the same as what the witness has
			mockWitness.On("ReportEvidence", mock.Anything, mock.MatchedBy(func(evidence types.Evidence) bool {
				evAgainstPrimary := &types.LightClientAttackEvidence{
					ConflictingBlock: &types.LightBlock{
						SignedHeader: primaryHeaders[testCase.divergenceHeight],
						ValidatorSet: primaryValidators[testCase.divergenceHeight],
					},
					CommonHeight: testCase.divergenceHeight,
				}
				return bytes.Equal(evidence.Hash(), evAgainstPrimary.Hash())
			})).Return(nil)
			mockPrimary.On("ReportEvidence", mock.Anything, mock.MatchedBy(func(evidence types.Evidence) bool {
				evAgainstWitness := &types.LightClientAttackEvidence{
					ConflictingBlock: &types.LightBlock{
						SignedHeader: witnessHeaders[testCase.divergenceHeight],
						ValidatorSet: witnessValidators[testCase.divergenceHeight],
					},
					CommonHeight: testCase.divergenceHeight,
				}
				return bytes.Equal(evidence.Hash(), evAgainstWitness.Hash())
			})).Return(nil)

			c, err := light.NewClient(
				ctx,
				chainID,
				light.TrustOptions{
					Period: 4 * time.Hour,
					Height: 1,
					Hash:   primaryHeaders[1].Hash(),
				},
				mockPrimary,
				[]provider.Provider{mockWitness},
				dbs.New(dbm.NewMemDB()),
				light.Logger(logger),
				testCase.lightOption,
			)
			require.NoError(t, err)

			// Check verification returns an error.
			_, err = c.VerifyLightBlockAtHeight(ctx, testCase.latestHeight, bTime.Add(300*time.Second))
			if assert.Error(t, err) {
				assert.Equal(t, light.ErrLightClientAttack, err)
			}

			mockWitness.AssertExpectations(t)
			mockPrimary.AssertExpectations(t)
		})
	}
}

func TestLightClientAttackEvidence_ForwardLunatic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	// primary performs a lunatic attack but changes the time of the header to
	// something in the future relative to the blockchain
	var (
		latestHeight      = int64(10)
		valSize           = 5
		forgedHeight      = int64(12)
		proofHeight       = int64(11)
		primaryHeaders    = make(map[int64]*types.SignedHeader, forgedHeight)
		primaryValidators = make(map[int64]*types.ValidatorSet, forgedHeight)
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := log.NewNopLogger()

	witnessHeaders, witnessValidators, chainKeys := genLightBlocksWithKeys(t, latestHeight, valSize, 2, bTime)
	for _, unusedHeader := range []int64{3, 5, 6, 8} {
		delete(witnessHeaders, unusedHeader)
	}

	// primary has the exact same headers except it forges one extra header in the future using keys from 2/5ths of
	// the validators
	for h := range witnessHeaders {
		primaryHeaders[h] = witnessHeaders[h]
		primaryValidators[h] = witnessValidators[h]
	}
	for _, unusedHeader := range []int64{3, 5, 6, 8} {
		delete(primaryHeaders, unusedHeader)
	}
	forgedKeys := chainKeys[latestHeight].ChangeKeys(3) // we change 3 out of the 5 validators (still 2/5 remain)
	primaryValidators[forgedHeight] = forgedKeys.ToValidators(2, 0)
	primaryHeaders[forgedHeight] = forgedKeys.GenSignedHeader(t,
		chainID,
		forgedHeight,
		bTime.Add(time.Duration(latestHeight+1)*time.Minute), // 11 mins
		nil,
		primaryValidators[forgedHeight],
		primaryValidators[forgedHeight],
		hash("app_hash"),
		hash("cons_hash"),
		hash("results_hash"),
		0, len(forgedKeys),
	)
	mockPrimary := mockNodeFromHeadersAndVals(primaryHeaders, primaryValidators)
	mockPrimary.On("ID").Return("mockPrimary")
	lastBlock, _ := mockPrimary.LightBlock(ctx, forgedHeight)
	mockPrimary.On("LightBlock", mock.Anything, int64(0)).Return(lastBlock, nil)
	mockPrimary.On("LightBlock", mock.Anything, mock.Anything).Return(nil, provider.ErrLightBlockNotFound)

	mockWitness := mockNodeFromHeadersAndVals(witnessHeaders, witnessValidators)
	mockWitness.On("ID").Return("mockWitness")
	lastBlock, _ = mockWitness.LightBlock(ctx, latestHeight)
	mockWitness.On("LightBlock", mock.Anything, int64(0)).Return(lastBlock, nil).Once()
	mockWitness.On("LightBlock", mock.Anything, int64(12)).Return(nil, provider.ErrHeightTooHigh)

	mockWitness.On("ReportEvidence", mock.Anything, mock.MatchedBy(func(evidence types.Evidence) bool {
		// Check evidence was sent to the witness against the full node
		evAgainstPrimary := &types.LightClientAttackEvidence{
			ConflictingBlock: &types.LightBlock{
				SignedHeader: primaryHeaders[forgedHeight],
				ValidatorSet: primaryValidators[forgedHeight],
			},
			CommonHeight: latestHeight,
		}
		return bytes.Equal(evidence.Hash(), evAgainstPrimary.Hash())
	})).Return(nil).Twice()

	// In order to perform the attack, the primary needs at least one accomplice as a witness to also
	// send the forged block
	accomplice := mockNodeFromHeadersAndVals(primaryHeaders, primaryValidators)
	accomplice.On("ID").Return("accomplice")
	lastBlock, _ = accomplice.LightBlock(ctx, forgedHeight)
	accomplice.On("LightBlock", mock.Anything, int64(0)).Return(lastBlock, nil)
	accomplice.On("LightBlock", mock.Anything, mock.Anything).Return(nil, provider.ErrLightBlockNotFound)

	c, err := light.NewClient(
		ctx,
		chainID,
		light.TrustOptions{
			Period: 4 * time.Hour,
			Height: 1,
			Hash:   primaryHeaders[1].Hash(),
		},
		mockPrimary,
		[]provider.Provider{mockWitness, accomplice},
		dbs.New(dbm.NewMemDB()),
		light.Logger(logger),
		light.MaxClockDrift(1*time.Second),
		light.MaxBlockLag(1*time.Second),
	)
	require.NoError(t, err)

	// two seconds later, the supporting withness should receive the header that can be used
	// to prove that there was an attack
	vals := chainKeys[latestHeight].ToValidators(2, 0)
	newLb := &types.LightBlock{
		SignedHeader: chainKeys[latestHeight].GenSignedHeader(t,
			chainID,
			proofHeight,
			bTime.Add(time.Duration(proofHeight+1)*time.Minute), // 12 mins
			nil,
			vals,
			vals,
			hash("app_hash"),
			hash("cons_hash"),
			hash("results_hash"),
			0, len(chainKeys),
		),
		ValidatorSet: vals,
	}
	go func() {
		time.Sleep(2 * time.Second)
		mockWitness.On("LightBlock", mock.Anything, int64(0)).Return(newLb, nil)
	}()

	// Now assert that verification returns an error. We craft the light clients time to be a little ahead of the chain
	// to allow a window for the attack to manifest itself.
	_, err = c.Update(ctx, bTime.Add(time.Duration(forgedHeight)*time.Minute))
	if assert.Error(t, err) {
		assert.Equal(t, light.ErrLightClientAttack, err)
	}

	// We attempt the same call but now the supporting witness has a block which should
	// immediately conflict in time with the primary
	_, err = c.VerifyLightBlockAtHeight(ctx, forgedHeight, bTime.Add(time.Duration(forgedHeight)*time.Minute))
	if assert.Error(t, err) {
		assert.Equal(t, light.ErrLightClientAttack, err)
	}

	// Lastly we test the unfortunate case where the light clients supporting witness doesn't update
	// in enough time
	mockLaggingWitness := mockNodeFromHeadersAndVals(witnessHeaders, witnessValidators)
	mockLaggingWitness.On("ID").Return("mockLaggingWitness")
	mockLaggingWitness.On("LightBlock", mock.Anything, int64(12)).Return(nil, provider.ErrHeightTooHigh)
	lastBlock, _ = mockLaggingWitness.LightBlock(ctx, latestHeight)
	mockLaggingWitness.On("LightBlock", mock.Anything, int64(0)).Return(lastBlock, nil)
	c, err = light.NewClient(
		ctx,
		chainID,
		light.TrustOptions{
			Period: 4 * time.Hour,
			Height: 1,
			Hash:   primaryHeaders[1].Hash(),
		},
		mockPrimary,
		[]provider.Provider{mockLaggingWitness, accomplice},
		dbs.New(dbm.NewMemDB()),
		light.Logger(logger),
		light.MaxClockDrift(1*time.Second),
		light.MaxBlockLag(1*time.Second),
	)
	require.NoError(t, err)

	_, err = c.Update(ctx, bTime.Add(time.Duration(forgedHeight)*time.Minute))
	assert.NoError(t, err)
	mockPrimary.AssertExpectations(t)
	mockWitness.AssertExpectations(t)
}

// 1. Different nodes therefore a divergent header is produced.
// => light client returns an error upon creation because primary and witness
// have a different view.
func TestClientDivergentTraces1(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	headers, vals, _ := genLightBlocksWithKeys(t, 1, 5, 2, bTime)
	mockPrimary := mockNodeFromHeadersAndVals(headers, vals)
	mockPrimary.On("ID").Return("mockPrimary")

	firstBlock, err := mockPrimary.LightBlock(ctx, 1)
	require.NoError(t, err)
	headers, vals, _ = genLightBlocksWithKeys(t, 1, 5, 2, bTime)
	mockWitness := mockNodeFromHeadersAndVals(headers, vals)
	mockWitness.On("ID").Return("mockWitness")

	logger := log.NewNopLogger()

	_, err = light.NewClient(
		ctx,
		chainID,
		light.TrustOptions{
			Height: 1,
			Hash:   firstBlock.Hash(),
			Period: 4 * time.Hour,
		},
		mockPrimary,
		[]provider.Provider{mockWitness},
		dbs.New(dbm.NewMemDB()),
		light.Logger(logger),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match primary")
	mockWitness.AssertExpectations(t)
	mockPrimary.AssertExpectations(t)
}

// 2. Two out of three nodes don't respond but the third has a header that matches
// => verification should be successful and all the witnesses should remain
func TestClientDivergentTraces2(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := log.NewNopLogger()

	headers, vals, _ := genLightBlocksWithKeys(t, 2, 5, 2, bTime)
	mockPrimaryNode := mockNodeFromHeadersAndVals(headers, vals)
	mockPrimaryNode.On("ID").Return("mockPrimaryNode")

	mockGoodWitness := mockNodeFromHeadersAndVals(headers, vals)
	mockGoodWitness.On("ID").Return("mockGoodWitness")

	mockDeadNode1 := &provider_mocks.Provider{}
	mockDeadNode1.On("ID").Return("mockDeadNode1")
	mockDeadNode1.On("LightBlock", mock.Anything, mock.Anything).Return(nil, provider.ErrNoResponse)

	mockDeadNode2 := &provider_mocks.Provider{}
	mockDeadNode2.On("ID").Return("mockDeadNode2")
	mockDeadNode2.On("LightBlock", mock.Anything, mock.Anything).Return(nil, provider.ErrNoResponse)

	firstBlock, err := mockPrimaryNode.LightBlock(ctx, 1)
	require.NoError(t, err)
	c, err := light.NewClient(
		ctx,
		chainID,
		light.TrustOptions{
			Height: 1,
			Hash:   firstBlock.Hash(),
			Period: 4 * time.Hour,
		},
		mockPrimaryNode,
		[]provider.Provider{mockDeadNode1, mockDeadNode2, mockGoodWitness},
		dbs.New(dbm.NewMemDB()),
		light.Logger(logger),
	)
	require.NoError(t, err)

	_, err = c.VerifyLightBlockAtHeight(ctx, 2, bTime.Add(1*time.Hour))
	assert.NoError(t, err)
	assert.Equal(t, 3, len(c.Witnesses()))
	mockDeadNode1.AssertExpectations(t)
	mockPrimaryNode.AssertExpectations(t)
}

// 3. witness has the same first header, but different second header
// => creation should succeed, but the verification should fail
//nolint: dupl
func TestClientDivergentTraces3(t *testing.T) {
	logger := log.NewNopLogger()

	//
	primaryHeaders, primaryVals, _ := genLightBlocksWithKeys(t, 2, 5, 2, bTime)
	mockPrimary := mockNodeFromHeadersAndVals(primaryHeaders, primaryVals)
	mockPrimary.On("ID").Return("mockPrimary")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	firstBlock, err := mockPrimary.LightBlock(ctx, 1)
	require.NoError(t, err)

	mockHeaders, mockVals, _ := genLightBlocksWithKeys(t, 2, 5, 2, bTime)
	mockHeaders[1] = primaryHeaders[1]
	mockVals[1] = primaryVals[1]
	mockWitness := mockNodeFromHeadersAndVals(mockHeaders, mockVals)
	mockWitness.On("ID").Return("mockWitness")

	c, err := light.NewClient(
		ctx,
		chainID,
		light.TrustOptions{
			Height: 1,
			Hash:   firstBlock.Hash(),
			Period: 4 * time.Hour,
		},
		mockPrimary,
		[]provider.Provider{mockWitness},
		dbs.New(dbm.NewMemDB()),
		light.Logger(logger),
	)
	require.NoError(t, err)

	_, err = c.VerifyLightBlockAtHeight(ctx, 2, bTime.Add(1*time.Hour))
	assert.Error(t, err)
	assert.Equal(t, 1, len(c.Witnesses()))
	mockWitness.AssertExpectations(t)
	mockPrimary.AssertExpectations(t)
}

// 4. Witness has a divergent header but can not produce a valid trace to back it up.
// It should be ignored
//nolint: dupl
func TestClientDivergentTraces4(t *testing.T) {
	logger := log.NewNopLogger()

	//
	primaryHeaders, primaryVals, _ := genLightBlocksWithKeys(t, 2, 5, 2, bTime)
	mockPrimary := mockNodeFromHeadersAndVals(primaryHeaders, primaryVals)
	mockPrimary.On("ID").Return("mockPrimary")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	firstBlock, err := mockPrimary.LightBlock(ctx, 1)
	require.NoError(t, err)

	witnessHeaders, witnessVals, _ := genLightBlocksWithKeys(t, 2, 5, 2, bTime)
	primaryHeaders[2] = witnessHeaders[2]
	primaryVals[2] = witnessVals[2]
	mockWitness := mockNodeFromHeadersAndVals(primaryHeaders, primaryVals)
	mockWitness.On("ID").Return("mockWitness")

	c, err := light.NewClient(
		ctx,
		chainID,
		light.TrustOptions{
			Height: 1,
			Hash:   firstBlock.Hash(),
			Period: 4 * time.Hour,
		},
		mockPrimary,
		[]provider.Provider{mockWitness},
		dbs.New(dbm.NewMemDB()),
		light.Logger(logger),
	)
	require.NoError(t, err)

	_, err = c.VerifyLightBlockAtHeight(ctx, 2, bTime.Add(1*time.Hour))
	assert.Error(t, err)
	assert.Equal(t, 1, len(c.Witnesses()))
	mockWitness.AssertExpectations(t)
	mockPrimary.AssertExpectations(t)
}
