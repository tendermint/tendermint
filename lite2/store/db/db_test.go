package db

import (
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
	err = dbStore.SaveSignedHeaderAndNextValidatorSet(
		&types.SignedHeader{Header: &types.Header{Height: 1}}, &types.ValidatorSet{})
	require.NoError(t, err)

	height, err = dbStore.LastSignedHeaderHeight()
	require.NoError(t, err)
	assert.EqualValues(t, 1, height)

	height, err = dbStore.FirstSignedHeaderHeight()
	require.NoError(t, err)
	assert.EqualValues(t, 1, height)
}

func Test_SaveSignedHeaderAndNextValidatorSet(t *testing.T) {
	dbStore := New(dbm.NewMemDB(), "Test_SaveSignedHeaderAndNextValidatorSet")

	// Empty store
	h, err := dbStore.SignedHeader(1)
	require.Error(t, err)
	assert.Nil(t, h)

	valSet, err := dbStore.ValidatorSet(2)
	require.Error(t, err)
	assert.Nil(t, valSet)

	// 1 key
	err = dbStore.SaveSignedHeaderAndNextValidatorSet(
		&types.SignedHeader{Header: &types.Header{Height: 1}}, &types.ValidatorSet{})
	require.NoError(t, err)

	h, err = dbStore.SignedHeader(1)
	require.NoError(t, err)
	assert.NotNil(t, h)

	valSet, err = dbStore.ValidatorSet(2)
	require.NoError(t, err)
	assert.NotNil(t, valSet)

	// Empty store
	err = dbStore.DeleteSignedHeaderAndNextValidatorSet(1)
	require.NoError(t, err)

	h, err = dbStore.SignedHeader(1)
	require.Error(t, err)
	assert.Nil(t, h)

	valSet, err = dbStore.ValidatorSet(2)
	require.Error(t, err)
	assert.Nil(t, valSet)
}

func Test_SignedHeaderAfter(t *testing.T) {
	dbStore := New(dbm.NewMemDB(), "Test_SignedHeaderAfter")

	assert.Panics(t, func() {
		dbStore.SignedHeaderAfter(0)
		dbStore.SignedHeaderAfter(100)
	})

	err := dbStore.SaveSignedHeaderAndNextValidatorSet(
		&types.SignedHeader{Header: &types.Header{Height: 2}}, &types.ValidatorSet{})
	require.NoError(t, err)

	h, err := dbStore.SignedHeaderAfter(1)
	require.NoError(t, err)
	if assert.NotNil(t, h) {
		assert.EqualValues(t, 2, h.Height)
	}
}
