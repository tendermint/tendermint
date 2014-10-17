package merkle

import (
	. "github.com/tendermint/tendermint/common"

	"testing"
)

// TODO: Actually test. All this does is help debug.
// consensus/part_set tests some of this functionality.
func TestHashTreeMerkleTrail(t *testing.T) {

	numHashes := 5

	// Make some fake "hashes".
	hashes := make([][]byte, numHashes)
	for i := 0; i < numHashes; i++ {
		hashes[i] = RandBytes(32)
		t.Logf("hash %v\t%X\n", i, hashes[i])
	}

	hashTree := HashTreeFromHashes(hashes)
	for i := 0; i < len(hashTree); i++ {
		t.Logf("tree %v\t%X\n", i, hashTree[i])
	}

	for i := 0; i < numHashes; i++ {
		t.Logf("trail %v\n", i)
		trail := HashTrailForIndex(hashTree, i)
		for j := 0; j < len(trail); j++ {
			t.Logf("index: %v, hash: %X\n", j, trail[j])
		}
	}

}
