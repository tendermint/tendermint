package merkle

import (
	"bytes"

	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/common/test"

	"fmt"
	"testing"
)

type testItem []byte

func (tI testItem) Hash() []byte {
	return []byte(tI)
}

func TestSimpleProof(t *testing.T) {

	numItems := 100

	items := make([]Hashable, numItems)
	for i := 0; i < numItems; i++ {
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

func TestKVPairs(t *testing.T) {
	// NOTE: in alphabetical order for convenience.
	m := map[string]interface{}{}
	m["bytez"] = []byte("hizz") // 0
	m["light"] = "shadow"       // 1
	m["one"] = 1                // 2
	m["one_u64"] = uint64(1)    // 3
	m["struct"] = struct {      // 4
		A int
		B int
	}{0, 1}

	kvPairsH := MakeSortedKVPairs(m)
	// rootHash := SimpleHashFromHashables(kvPairsH)
	proofs := SimpleProofsFromHashables(kvPairsH)

	// Some manual tests
	if !bytes.Equal(proofs[1].LeafHash, KVPair{"light", "shadow"}.Hash()) {
		t.Errorf("\"light\": proof failed")
		fmt.Printf("%v\n%X", proofs[0], KVPair{"light", "shadow"}.Hash())
	}
	if !bytes.Equal(proofs[2].LeafHash, KVPair{"one", 1}.Hash()) {
		t.Errorf("\"one\": proof failed")
	}
	if !bytes.Equal(proofs[4].LeafHash, KVPair{"struct", struct {
		A int
		B int
	}{0, 1}}.Hash()) {
		t.Errorf("\"struct\": proof failed")
	}

}
