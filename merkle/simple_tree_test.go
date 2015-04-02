package merkle

import (
	. "github.com/tendermint/tendermint/common"

	"bytes"
	"testing"
)

type testItem []byte

func (tI testItem) Hash() []byte {
	return []byte(tI)
}

func TestMerkleTrails(t *testing.T) {

	numItems := uint(100)

	items := make([]Hashable, numItems)
	for i := uint(0); i < numItems; i++ {
		items[i] = testItem(RandBytes(32))
	}

	root := HashFromHashables(items)

	trails, rootTrail := HashTrailsFromHashables(items)

	// Assert that HashFromHashables and HashTrailsFromHashables are compatible.
	if !bytes.Equal(root, rootTrail.Hash) {
		t.Errorf("Root mismatch:\n%X vs\n%X", root, rootTrail.Hash)
	}

	// For each item, check the trail.
	for i, item := range items {
		itemHash := item.Hash()
		flatTrail := trails[i].Flatten()

		// Verify success
		ok := VerifyHashTrail(uint(i), numItems, itemHash, flatTrail, root)
		if !ok {
			t.Errorf("Verification failed for index %v.", i)
		}

		// Wrong item index should make it fail
		ok = VerifyHashTrail(uint(i)+1, numItems, itemHash, flatTrail, root)
		if ok {
			t.Errorf("Expected verification to fail for wrong index %v.", i)
		}

		// Trail too long should make it fail
		trail2 := append(flatTrail, RandBytes(32))
		ok = VerifyHashTrail(uint(i), numItems, itemHash, trail2, root)
		if ok {
			t.Errorf("Expected verification to fail for wrong trail length.")
		}

		// Trail too short should make it fail
		trail2 = flatTrail[:len(flatTrail)-1]
		ok = VerifyHashTrail(uint(i), numItems, itemHash, trail2, root)
		if ok {
			t.Errorf("Expected verification to fail for wrong trail length.")
		}

		// Mutating the itemHash should make it fail.
		itemHash2 := make([]byte, len(itemHash))
		copy(itemHash2, itemHash)
		itemHash2[0] += byte(0x01)
		ok = VerifyHashTrail(uint(i), numItems, itemHash2, flatTrail, root)
		if ok {
			t.Errorf("Expected verification to fail for mutated leaf hash")
		}
	}
}
