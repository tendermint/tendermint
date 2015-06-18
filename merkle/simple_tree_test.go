package merkle

import (
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/common/test"

	"testing"
)

type testItem []byte

func (tI testItem) Hash() []byte {
	return []byte(tI)
}

func TestSimpleProof(t *testing.T) {

	numItems := uint(100)

	items := make([]Hashable, numItems)
	for i := uint(0); i < numItems; i++ {
		items[i] = testItem(RandBytes(32))
	}

	rootHash := SimpleHashFromHashables(items)

	proofs := SimpleProofsFromHashables(items)

	// For each item, check the trail.
	for i, item := range items {
		itemHash := item.Hash()
		proof := proofs[i]

		// Verify success
		ok := proof.Verify(itemHash, rootHash)
		if !ok {
			t.Errorf("Verification failed for index %v.", i)
		}

		// Wrong item index should make it fail
		proof.Index += 1
		{
			ok = proof.Verify(itemHash, rootHash)
			if ok {
				t.Errorf("Expected verification to fail for wrong index %v.", i)
			}
		}
		proof.Index -= 1

		// Trail too long should make it fail
		origInnerHashes := proof.InnerHashes
		proof.InnerHashes = append(proof.InnerHashes, RandBytes(32))
		{
			ok = proof.Verify(itemHash, rootHash)
			if ok {
				t.Errorf("Expected verification to fail for wrong trail length.")
			}
		}
		proof.InnerHashes = origInnerHashes

		// Trail too short should make it fail
		proof.InnerHashes = proof.InnerHashes[0 : len(proof.InnerHashes)-1]
		{
			ok = proof.Verify(itemHash, rootHash)
			if ok {
				t.Errorf("Expected verification to fail for wrong trail length.")
			}
		}
		proof.InnerHashes = origInnerHashes

		// Mutating the itemHash should make it fail.
		ok = proof.Verify(MutateByteSlice(itemHash), rootHash)
		if ok {
			t.Errorf("Expected verification to fail for mutated leaf hash")
		}

		// Mutating the rootHash should make it fail.
		ok = proof.Verify(itemHash, MutateByteSlice(rootHash))
		if ok {
			t.Errorf("Expected verification to fail for mutated root hash")
		}
	}
}
