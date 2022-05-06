//go:build gofuzz || go1.18

package tests

import (
	"bytes"
	"testing"

	tmcrypto "github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/merkle"
	tmrand "github.com/tendermint/tendermint/libs/rand"
)

func FuzzProofsFromByteSlices(f *testing.F) {
	seeds := [][]byte{
		nil,
		{},
	}

	// [][]byte isn't supported as an input hence add []byte values.
	var fakeSeparator = []byte("***FAKE_MARKER***")

	for i := 0; i < 10; i++ {
		total := 100
		seed := make([][]byte, total)
		for j := 0; j < total; j++ {
			seed[j] = tmrand.Bytes(tmcrypto.HashSize)
		}
		seeds = append(seeds, bytes.Join(seed, fakeSeparator))
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, joinedInput []byte) {
		input := bytes.Split(joinedInput, fakeSeparator)
		if len(input) == 1 {
			if joinedInput == nil {
				input = nil
			} else {
				input = [][]byte{}
			}
		}
		_, _ = merkle.ProofsFromByteSlices(input)
	})
}
