package merkle

import (
	"bytes"
	"crypto/rand"

	cmn "github.com/tendermint/tendermint/libs/common"
	. "github.com/tendermint/tendermint/libs/test"

	"testing"
)

func TestSimpleProof(t *testing.T) {

	total := 100
	sliceSize := 100

	items := make([][]byte, total)
	for i := 0; i < total; i++ {
		items[i] = make([]byte, sliceSize)
		rand.Read(items[i])
	}

	rootHash := SimpleHashFromByteSlices(items)

	rootHash2, proofs := SimpleProofsFromByteSlices(items)

	if !bytes.Equal(rootHash, rootHash2) {
		t.Errorf("Unmatched root hashes: %X vs %X", rootHash, rootHash2)
	}

	// For each item, check the trail.
	for i, item := range items {
		proof := proofs[i]

		// Verify success
		ok := proof.Verify(i, total, item, rootHash)
		if !ok {
			t.Errorf("Verification failed for index %v.", i)
		}

		// Wrong item index should make it fail
		{
			ok = proof.Verify((i+1)%total, total, item, rootHash)
			if ok {
				t.Errorf("Expected verification to fail for wrong index %v.", i)
			}
		}

		// Trail too long should make it fail
		origAunts := proof.Aunts
		proof.Aunts = append(proof.Aunts, cmn.RandBytes(32))
		{
			ok = proof.Verify(i, total, item, rootHash)
			if ok {
				t.Errorf("Expected verification to fail for wrong trail length.")
			}
		}
		proof.Aunts = origAunts

		// Trail too short should make it fail
		proof.Aunts = proof.Aunts[0 : len(proof.Aunts)-1]
		{
			ok = proof.Verify(i, total, item, rootHash)
			if ok {
				t.Errorf("Expected verification to fail for wrong trail length.")
			}
		}
		proof.Aunts = origAunts

		// Mutating the itemHash should make it fail.
		ok = proof.Verify(i, total, MutateByteSlice(item), rootHash)
		if ok {
			t.Errorf("Expected verification to fail for mutated leaf hash")
		}

		// Mutating the rootHash should make it fail.
		ok = proof.Verify(i, total, item, MutateByteSlice(rootHash))
		if ok {
			t.Errorf("Expected verification to fail for mutated root hash")
		}
	}
}
