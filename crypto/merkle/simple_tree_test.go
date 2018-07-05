package merkle

import (
	"bytes"

	cmn "github.com/tendermint/tendermint/libs/common"
	. "github.com/tendermint/tendermint/libs/test"

	"github.com/tendermint/tendermint/crypto/tmhash"
	"testing"
)

type testItem []byte

func (tI testItem) Hash() []byte {
	return []byte(tI)
}

func TestSimpleProof(t *testing.T) {

	total := 100

	items := make([]Hasher, total)
	for i := 0; i < total; i++ {
		items[i] = testItem(cmn.RandBytes(tmhash.Size))
	}

	rootHash := SimpleHashFromHashers(items)

	rootHash2, proofs := SimpleProofsFromHashers(items)

	if !bytes.Equal(rootHash, rootHash2) {
		t.Errorf("Unmatched root hashes: %X vs %X", rootHash, rootHash2)
	}

	// For each item, check the trail.
	for i, item := range items {
		itemHash := item.Hash()
		proof := proofs[i]

		// Verify success
		err := proof.Verify(rootHash, i, total, itemHash)
		if err != nil {
			t.Errorf("Verification failed: %v.", err)
		}

		// Wrong item index should make it fail
		{
			err = proof.Verify(rootHash, (i+1)%total, total, itemHash)
			if err == nil {
				t.Errorf("Expected verification to fail for wrong index %v", i)
			}
		}

		// Trail too long should make it fail
		origAunts := proof.Aunts
		proof.Aunts = append(proof.Aunts, cmn.RandBytes(32))
		{
			err = proof.Verify(rootHash, i, total, itemHash)
			if err == nil {
				t.Errorf("Expected verification to fail for wrong trail length")
			}
		}
		proof.Aunts = origAunts

		// Trail too short should make it fail
		proof.Aunts = proof.Aunts[0 : len(proof.Aunts)-1]
		{
			err = proof.Verify(rootHash, i, total, itemHash)
			if err == nil {
				t.Errorf("Expected verification to fail for wrong trail length")
			}
		}
		proof.Aunts = origAunts

		// Mutating the itemHash should make it fail.
		err = proof.Verify(rootHash, i, total, MutateByteSlice(itemHash))
		if err == nil {
			t.Errorf("Expected verification to fail for mutated leaf hash")
		}

		// Mutating the rootHash should make it fail.
		err = proof.Verify(MutateByteSlice(rootHash), i, total, itemHash)
		if err == nil {
			t.Errorf("Expected verification to fail for mutated root hash")
		}
	}
}
