package tmhash_test

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/tmhash"
)

func TestHash(t *testing.T) {
	testVector := []byte("abc")
	hasher := tmhash.New()
	_, err := hasher.Write(testVector)
	require.NoError(t, err)
	bz := hasher.Sum(nil)

	bz2 := tmhash.Sum(testVector)

	hasher = sha256.New()
	_, err = hasher.Write(testVector)
	require.NoError(t, err)
	bz3 := hasher.Sum(nil)

	assert.Equal(t, bz, bz2)
	assert.Equal(t, bz, bz3)
}
