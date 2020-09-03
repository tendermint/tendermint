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
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func TestLightClientAttackEvidence_Lunatic(t *testing.T) {
	// fullNode2 is primary and performs a lunatic attack
	latestHeight := int64(10)
	valSize := 5
	witnessHeaders, witnessValidators, chainKeys := genMockNodeWithKeys(chainID, latestHeight, valSize, 1, bTime)
	witness := mockp.New(chainID, witnessHeaders, witnessValidators)
	divergenceHeight := int64(6)
	primaryHeaders := make(map[int64]*types.SignedHeader, latestHeight)
	primaryValidators := make(map[int64]*types.ValidatorSet, latestHeight)
	forgedKeys := chainKeys[divergenceHeight - 1].ChangeKeys(3) // we change 3 out of the 5 validators (still 1/5 remain)
	forgedVals := forgedKeys.ToValidators(2, 0)
	for height := int64(1); height < latestHeight; height++ {
		if height < divergenceHeight {
			primaryHeaders[height] = witnessHeaders[height]
			primaryValidators[height] = witnessValidators[height]
			continue
		}
		primaryHeaders[height] = forgedKeys.GenSignedHeader(chainID, height, bTime.Add(time.Duration(height)*time.Minute), nil, forgedVals, forgedVals,
		hash("app_hash"), hash("cons_hash"), hash("results_hash"), 0, len(keys))
		primaryValidators[height] = forgedVals
	}
	primary := mockp.New(chainID, primaryHeaders, primaryValidators)

	c, err := light.NewClient(
		chainID,
		light.TrustOptions{
			Period: 4 * time.Hour,
			Height: 1,
			Hash: primaryHeaders[1].Hash(),
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
		ConflictingBlock: &types.LightBlock{
			SignedHeader: primaryHeaders[divergenceHeight],
			ValidatorSet: forgedVals,
		},
		CommonHeight: 5,
		Timestamp: bTime.Add(5 * time.Minute),
		AttackType: tmproto.LightClientAttackType_LUNATIC,
	}
	assert.True(t, witness.HasEvidence(evAgainstPrimary))
}

func TestClientDivergentTraces(t *testing.T) {
	primary := mockp.New(genMockNode(chainID, 10, 5, 2, bTime))
	firstBlock, _ := primary.LightBlock(1)
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
	assert.Equal(t, 1, len(c.Witnesses()))
}
