package db

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/types"
)

func TestLast_FirstSignedHeaderHeight(t *testing.T) {
	dbStore := New(dbm.NewMemDB(), "TestLast_FirstSignedHeaderHeight")

	// Empty store
	height, err := dbStore.LastSignedHeaderHeight()
	require.NoError(t, err)
	assert.EqualValues(t, -1, height)

	height, err = dbStore.FirstSignedHeaderHeight()
	require.NoError(t, err)
	assert.EqualValues(t, -1, height)

	// 1 key
	err = dbStore.SaveSignedHeaderAndValidatorSet(
		&types.SignedHeader{Header: &types.Header{Height: 1}}, &types.ValidatorSet{})
	require.NoError(t, err)

	height, err = dbStore.LastSignedHeaderHeight()
	require.NoError(t, err)
	assert.EqualValues(t, 1, height)

	height, err = dbStore.FirstSignedHeaderHeight()
	require.NoError(t, err)
	assert.EqualValues(t, 1, height)
}

func Test_SaveSignedHeaderAndValidatorSet(t *testing.T) {
	dbStore := New(dbm.NewMemDB(), "Test_SaveSignedHeaderAndValidatorSet")

	// Empty store
	h, err := dbStore.SignedHeader(1)
	require.Error(t, err)
	assert.Nil(t, h)

	valSet, err := dbStore.ValidatorSet(1)
	require.Error(t, err)
	assert.Nil(t, valSet)

	// 1 key
	err = dbStore.SaveSignedHeaderAndValidatorSet(
		&types.SignedHeader{Header: &types.Header{Height: 1}}, &types.ValidatorSet{})
	require.NoError(t, err)

	h, err = dbStore.SignedHeader(1)
	require.NoError(t, err)
	assert.NotNil(t, h)

	valSet, err = dbStore.ValidatorSet(1)
	require.NoError(t, err)
	assert.NotNil(t, valSet)

	// Empty store
	err = dbStore.DeleteSignedHeaderAndValidatorSet(1)
	require.NoError(t, err)

	h, err = dbStore.SignedHeader(1)
	require.Error(t, err)
	assert.Nil(t, h)

	valSet, err = dbStore.ValidatorSet(1)
	require.Error(t, err)
	assert.Nil(t, valSet)
}

func Test_SignedHeaderBefore(t *testing.T) {
	dbStore := New(dbm.NewMemDB(), "Test_SignedHeaderBefore")

	assert.Panics(t, func() {
		_, _ = dbStore.SignedHeaderBefore(0)
		_, _ = dbStore.SignedHeaderBefore(100)
	})

	err := dbStore.SaveSignedHeaderAndValidatorSet(
		&types.SignedHeader{Header: &types.Header{Height: 2}}, &types.ValidatorSet{})
	require.NoError(t, err)

	h, err := dbStore.SignedHeaderBefore(3)
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
	err = dbStore.SaveSignedHeaderAndValidatorSet(
		&types.SignedHeader{Header: &types.Header{Height: 2}}, &types.ValidatorSet{})
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
		err = dbStore.SaveSignedHeaderAndValidatorSet(
			&types.SignedHeader{Header: &types.Header{Height: int64(i)}}, &types.ValidatorSet{})
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

			dbStore.SaveSignedHeaderAndValidatorSet(
				&types.SignedHeader{Header: &types.Header{Height: i}}, &types.ValidatorSet{})

			dbStore.SignedHeader(i)
			dbStore.ValidatorSet(i)
			dbStore.LastSignedHeaderHeight()
			dbStore.FirstSignedHeaderHeight()

			dbStore.Prune(2)
			_ = dbStore.Size()

			dbStore.DeleteSignedHeaderAndValidatorSet(1)
		}(int64(i))
	}

	wg.Wait()
}
