package light_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/light/provider"
	mockp "github.com/tendermint/tendermint/light/provider/mock"
	dbs "github.com/tendermint/tendermint/light/store/db"
	"github.com/tendermint/tendermint/types"
)

func TestLightClientAttackEvidence_Lunatic(t *testing.T) {
	// primary performs a lunatic attack
	var (
		latestHeight      = int64(10)
		valSize           = 5
		divergenceHeight  = int64(6)
		primaryHeaders    = make(map[int64]*types.SignedHeader, latestHeight)
		primaryValidators = make(map[int64]*types.ValidatorSet, latestHeight)
	)

	witnessHeaders, witnessValidators, chainKeys := genMockNodeWithKeys(chainID, latestHeight, valSize, 2, bTime)
	witness := mockp.New(chainID, witnessHeaders, witnessValidators)
	forgedKeys := chainKeys[divergenceHeight-1].ChangeKeys(3) // we change 3 out of the 5 validators (still 2/5 remain)
	forgedVals := forgedKeys.ToValidators(2, 0)

	for height := int64(1); height <= latestHeight; height++ {
		if height < divergenceHeight {
			primaryHeaders[height] = witnessHeaders[height]
			primaryValidators[height] = witnessValidators[height]
			continue
		}
		primaryHeaders[height] = forgedKeys.GenSignedHeader(chainID, height, bTime.Add(time.Duration(height)*time.Minute),
			nil, forgedVals, forgedVals, hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(forgedKeys))
		primaryValidators[height] = forgedVals
	}
	primary := mockp.New(chainID, primaryHeaders, primaryValidators)

	c, err := light.NewClient(
		chainID,
		light.TrustOptions{
			Period: 4 * time.Hour,
			Height: 1,
			Hash:   primaryHeaders[1].Hash(),
		},
		primary,
		[]provider.Provider{witness},
		dbs.New(dbm.NewMemDB(), chainID),
		light.Logger(log.TestingLogger()),
		light.MaxRetryAttempts(1),
	)
	require.NoError(t, err)

	// Check verification returns an error.
	_, err = c.VerifyLightBlockAtHeight(10, bTime.Add(1*time.Hour))
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "does not match primary")
	}

	// Check evidence was sent to both full nodes.
	evAgainstPrimary := &types.LightClientAttackEvidence{
		// after the divergence height the valset doesn't change so we expect the evidence to be for height 10
		ConflictingBlock: &types.LightBlock{
			SignedHeader: primaryHeaders[10],
			ValidatorSet: primaryValidators[10],
		},
		CommonHeight: 4,
	}
	assert.True(t, witness.HasEvidence(evAgainstPrimary))

	evAgainstWitness := &types.LightClientAttackEvidence{
		// when forming evidence against witness we learn that the canonical chain continued to change validator sets
		// hence the conflicting block is at 7
		ConflictingBlock: &types.LightBlock{
			SignedHeader: witnessHeaders[7],
			ValidatorSet: witnessValidators[7],
		},
		CommonHeight: 4,
	}
	assert.True(t, primary.HasEvidence(evAgainstWitness))
}

func TestLightClientAttackEvidence_Equivocation(t *testing.T) {
	// primary performs an equivocation attack
	var (
		latestHeight      = int64(10)
		valSize           = 5
		divergenceHeight  = int64(6)
		primaryHeaders    = make(map[int64]*types.SignedHeader, latestHeight)
		primaryValidators = make(map[int64]*types.ValidatorSet, latestHeight)
	)
	// validators don't change in this network (however we still use a map just for convenience)
	witnessHeaders, witnessValidators, chainKeys := genMockNodeWithKeys(chainID, latestHeight+2, valSize, 2, bTime)
	witness := mockp.New(chainID, witnessHeaders, witnessValidators)

	for height := int64(1); height <= latestHeight; height++ {
		if height < divergenceHeight {
			primaryHeaders[height] = witnessHeaders[height]
			primaryValidators[height] = witnessValidators[height]
			continue
		}
		// we don't have a network partition so we will make 4/5 (greater than 2/3) malicious and vote again for
		// a different block (which we do by adding txs)
		primaryHeaders[height] = chainKeys[height].GenSignedHeader(chainID, height,
			bTime.Add(time.Duration(height)*time.Minute), []types.Tx{[]byte("abcd")},
			witnessValidators[height], witnessValidators[height+1], hash("app_hash"),
			hash("cons_hash"), hash("results_hash"), 0, len(chainKeys[height])-1)
		primaryValidators[height] = witnessValidators[height]
	}
	primary := mockp.New(chainID, primaryHeaders, primaryValidators)

	c, err := light.NewClient(
		chainID,
		light.TrustOptions{
			Period: 4 * time.Hour,
			Height: 1,
			Hash:   primaryHeaders[1].Hash(),
		},
		primary,
		[]provider.Provider{witness},
		dbs.New(dbm.NewMemDB(), chainID),
		light.Logger(log.TestingLogger()),
		light.MaxRetryAttempts(1),
	)
	require.NoError(t, err)

	// Check verification returns an error.
	_, err = c.VerifyLightBlockAtHeight(10, bTime.Add(1*time.Hour))
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "does not match primary")
	}

	// Check evidence was sent to both full nodes.
	// Common height should be set to the height of the divergent header in the instance
	// of an equivocation attack and the validator sets are the same as what the witness has
	evAgainstPrimary := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: primaryHeaders[divergenceHeight],
			ValidatorSet: primaryValidators[divergenceHeight],
		},
		CommonHeight: divergenceHeight,
	}
	assert.True(t, witness.HasEvidence(evAgainstPrimary))

	evAgainstWitness := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: witnessHeaders[divergenceHeight],
			ValidatorSet: witnessValidators[divergenceHeight],
		},
		CommonHeight: divergenceHeight,
	}
	assert.True(t, primary.HasEvidence(evAgainstWitness))
}

func TestClientDivergentTraces(t *testing.T) {
	primary := mockp.New(genMockNode(chainID, 10, 5, 2, bTime))
	firstBlock, err := primary.LightBlock(1)
	require.NoError(t, err)
	witness := mockp.New(genMockNode(chainID, 10, 5, 2, bTime))

	c, err := light.NewClient(
		chainID,
		light.TrustOptions{
			Height: 1,
			Hash:   firstBlock.Hash(),
			Period: 4 * time.Hour,
		},
		primary,
		[]provider.Provider{witness},
		dbs.New(dbm.NewMemDB(), chainID),
		light.Logger(log.TestingLogger()),
		light.MaxRetryAttempts(1),
	)
	require.NoError(t, err)

	// 1. Different nodes therefore a divergent header is produced but the
	// light client can't verify it because it has a different trusted header.
	_, err = c.VerifyLightBlockAtHeight(10, bTime.Add(1*time.Hour))
	assert.Error(t, err)
	assert.Equal(t, 0, len(c.Witnesses()))

	// 2. Two out of three nodes don't respond but the third has a header that matches
	// verification should be successful and all the witnesses should remain

	c, err = light.NewClient(
		chainID,
		light.TrustOptions{
			Height: 1,
			Hash:   firstBlock.Hash(),
			Period: 4 * time.Hour,
		},
		primary,
		[]provider.Provider{deadNode, deadNode, primary},
		dbs.New(dbm.NewMemDB(), chainID),
		light.Logger(log.TestingLogger()),
		light.MaxRetryAttempts(1),
	)
	require.NoError(t, err)
	_, err = c.VerifyLightBlockAtHeight(10, bTime.Add(1*time.Hour))
	assert.NoError(t, err)
	assert.Equal(t, 3, len(c.Witnesses()))
}
