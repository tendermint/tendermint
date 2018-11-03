package merkle

import (
	"testing"

	"github.com/stretchr/testify/require"

	cmn "github.com/tendermint/tendermint/libs/common"
	. "github.com/tendermint/tendermint/libs/test"

	"github.com/tendermint/tendermint/crypto/tmhash"
)

type testItem []byte

func (tI testItem) Hash() []byte {
	return []byte(tI)
}

func TestSimpleProof(t *testing.T) {

	total := 100

	items := make([][]byte, total)
	for i := 0; i < total; i++ {
		items[i] = testItem(cmn.RandBytes(tmhash.Size))
	}

	rootHash := SimpleHashFromByteSlices(items)

	rootHash2, proofs := SimpleProofsFromByteSlices(items)

	require.Equal(t, rootHash, rootHash2, "Unmatched root hashes: %X vs %X", rootHash, rootHash2)

	// For each item, check the trail.
	for i, item := range items {
		itemHash := tmhash.Sum(item)
		proof := proofs[i]

		// Check total/index
		require.Equal(t, proof.Index, i, "Unmatched indicies: %d vs %d", proof.Index, i)

		require.Equal(t, proof.Total, total, "Unmatched totals: %d vs %d", proof.Total, total)

		// Verify success
		err := proof.Verify(rootHash, itemHash)
		require.NoError(t, err, "Verificatior failed: %v.", err)

		// Trail too long should make it fail
		origAunts := proof.Aunts
		proof.Aunts = append(proof.Aunts, cmn.RandBytes(32))
		err = proof.Verify(rootHash, itemHash)
		require.Error(t, err, "Expected verification to fail for wrong trail length")

		proof.Aunts = origAunts

		// Trail too short should make it fail
		proof.Aunts = proof.Aunts[0 : len(proof.Aunts)-1]
		err = proof.Verify(rootHash, itemHash)
		require.Error(t, err, "Expected verification to fail for wrong trail length")

		proof.Aunts = origAunts

		// Mutating the itemHash should make it fail.
		err = proof.Verify(rootHash, MutateByteSlice(itemHash))
		require.Error(t, err, "Expected verification to fail for mutated leaf hash")

		// Mutating the rootHash should make it fail.
		err = proof.Verify(MutateByteSlice(rootHash), itemHash)
		require.Error(t, err, "Expected verification to fail for mutated root hash")
	}
}
