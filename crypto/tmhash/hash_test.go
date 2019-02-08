package tmhash_test

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/crypto/tmhash"
)

func TestHash(t *testing.T) {
	testVector := []byte("abc")
	hasher := tmhash.New()
	hasher.Write(testVector)
	bz := hasher.Sum(nil)

	bz2 := tmhash.Sum(testVector)

	hasher = sha256.New()
	hasher.Write(testVector)
	bz3 := hasher.Sum(nil)

	assert.Equal(t, bz, bz2)
	assert.Equal(t, bz, bz3)
}

func TestHashTruncated(t *testing.T) {
	testVector := []byte("abc")
	hasher := tmhash.NewTruncated()
	hasher.Write(testVector)
	bz := hasher.Sum(nil)

	bz2 := tmhash.SumTruncated(testVector)

	hasher = sha256.New()
	hasher.Write(testVector)
	bz3 := hasher.Sum(nil)
	bz3 = bz3[:tmhash.TruncatedSize]

	assert.Equal(t, bz, bz2)
	assert.Equal(t, bz, bz3)
}
