package db

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

func TestLast_FirstLightBlockHeight(t *testing.T) {
	dbStore := New(dbm.NewMemDB(), "TestLast_FirstLightBlockHeight")

	// Empty store
	height, err := dbStore.LastLightBlockHeight()
	require.NoError(t, err)
	assert.EqualValues(t, -1, height)

	height, err = dbStore.FirstLightBlockHeight()
	require.NoError(t, err)
	assert.EqualValues(t, -1, height)

	// 1 key
	err = dbStore.SaveLightBlock(randLightBlock(int64(1)))
	require.NoError(t, err)

	height, err = dbStore.LastLightBlockHeight()
	require.NoError(t, err)
	assert.EqualValues(t, 1, height)

	height, err = dbStore.FirstLightBlockHeight()
	require.NoError(t, err)
	assert.EqualValues(t, 1, height)
}

func Test_SaveLightBlock(t *testing.T) {
	dbStore := New(dbm.NewMemDB(), "Test_SaveLightBlockAndValidatorSet")

	// Empty store
	h, err := dbStore.LightBlock(1)
	require.Error(t, err)
	assert.Nil(t, h)

	// 1 key
	err = dbStore.SaveLightBlock(randLightBlock(1))
	require.NoError(t, err)

	size := dbStore.Size()
	assert.Equal(t, uint16(1), size)
	t.Log(size)

	h, err = dbStore.LightBlock(1)
	require.NoError(t, err)
	assert.NotNil(t, h)

	// Empty store
	err = dbStore.DeleteLightBlock(1)
	require.NoError(t, err)

	h, err = dbStore.LightBlock(1)
	require.Error(t, err)
	assert.Nil(t, h)

}

func Test_LightBlockBefore(t *testing.T) {
	dbStore := New(dbm.NewMemDB(), "Test_LightBlockBefore")

	assert.Panics(t, func() {
		_, _ = dbStore.LightBlockBefore(0)
		_, _ = dbStore.LightBlockBefore(100)
	})

	err := dbStore.SaveLightBlock(randLightBlock(int64(2)))
	require.NoError(t, err)

	h, err := dbStore.LightBlockBefore(3)
	require.NoError(t, err)
	if assert.NotNil(t, h) {
		assert.EqualValues(t, 2, h.Height)
	}
}

func Test_Prune(t *testing.T) {
	dbStore := New(dbm.NewMemDB(), "Test_Prune")

	// Empty store
	assert.EqualValues(t, 0, dbStore.Size())
	err := dbStore.Prune(0)
	require.NoError(t, err)

	// One header
	err = dbStore.SaveLightBlock(randLightBlock(2))
	require.NoError(t, err)

	assert.EqualValues(t, 1, dbStore.Size())

	err = dbStore.Prune(1)
	require.NoError(t, err)
	assert.EqualValues(t, 1, dbStore.Size())

	err = dbStore.Prune(0)
	require.NoError(t, err)
	assert.EqualValues(t, 0, dbStore.Size())

	// Multiple headers
	for i := 1; i <= 10; i++ {
		err = dbStore.SaveLightBlock(randLightBlock(int64(i)))
		require.NoError(t, err)
	}

	err = dbStore.Prune(11)
	require.NoError(t, err)
	assert.EqualValues(t, 10, dbStore.Size())

	err = dbStore.Prune(7)
	require.NoError(t, err)
	assert.EqualValues(t, 7, dbStore.Size())
}

func Test_Concurrency(t *testing.T) {
	dbStore := New(dbm.NewMemDB(), "Test_Prune")

	var wg sync.WaitGroup
	for i := 1; i <= 100; i++ {
		wg.Add(1)
		go func(i int64) {
			defer wg.Done()

			err := dbStore.SaveLightBlock(randLightBlock(i))
			require.NoError(t, err)

			_, err = dbStore.LightBlock(i)
			if err != nil {
				t.Log(err)
			}

			_, err = dbStore.LastLightBlockHeight()
			if err != nil {
				t.Log(err)
			}
			_, err = dbStore.FirstLightBlockHeight()
			if err != nil {
				t.Log(err)
			}

			err = dbStore.Prune(2)
			if err != nil {
				t.Log(err)
			}
			_ = dbStore.Size()

			err = dbStore.DeleteLightBlock(1)
			if err != nil {
				t.Log(err)
			}
		}(int64(i))
	}

	wg.Wait()
}

func randLightBlock(height int64) *types.LightBlock {
	vals, _ := types.GenerateValidatorSet(2)
	return &types.LightBlock{
		SignedHeader: &types.SignedHeader{
			Header: &types.Header{
				Version:            tmversion.Consensus{Block: version.BlockProtocol, App: 0},
				ChainID:            tmrand.Str(12),
				Height:             height,
				Time:               time.Now(),
				LastBlockID:        types.BlockID{},
				LastCommitHash:     crypto.CRandBytes(tmhash.Size),
				DataHash:           crypto.CRandBytes(tmhash.Size),
				ValidatorsHash:     crypto.CRandBytes(tmhash.Size),
				NextValidatorsHash: crypto.CRandBytes(tmhash.Size),
				ConsensusHash:      crypto.CRandBytes(tmhash.Size),
				AppHash:            crypto.CRandBytes(tmhash.Size),
				LastResultsHash:    crypto.CRandBytes(tmhash.Size),
				EvidenceHash:       crypto.CRandBytes(tmhash.Size),
				ProposerProTxHash:  crypto.CRandBytes(crypto.DefaultHashSize),
			},
			Commit: &types.Commit{},
		},
		ValidatorSet: vals,
	}
}
