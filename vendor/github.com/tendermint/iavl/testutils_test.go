package iavl

import (
	"fmt"
	mrand "math/rand"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/go-wire"
	. "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/db"
	. "github.com/tendermint/tmlibs/test"

	"runtime"
	"testing"
)

func dummyPathToKey(t *Tree, key []byte) *PathToKey {
	path, _, err := t.root.pathToKey(t, key)
	if err != nil {
		panic(err)
	}
	return path
}

func dummyLeafNode(key, val []byte) proofLeafNode {
	return proofLeafNode{key, val, 0}
}

func randstr(length int) string {
	return RandStr(length)
}

func i2b(i int) []byte {
	bz := make([]byte, 4)
	wire.PutInt32(bz, int32(i))
	return bz
}

func b2i(bz []byte) int {
	i := wire.GetInt32(bz)
	return int(i)
}

// Convenience for a new node
func N(l, r interface{}) *Node {
	var left, right *Node
	if _, ok := l.(*Node); ok {
		left = l.(*Node)
	} else {
		left = NewNode(i2b(l.(int)), nil)
	}
	if _, ok := r.(*Node); ok {
		right = r.(*Node)
	} else {
		right = NewNode(i2b(r.(int)), nil)
	}

	n := &Node{
		key:       right.lmd(nil).key,
		value:     nil,
		leftNode:  left,
		rightNode: right,
	}
	n.calcHeightAndSize(nil)
	return n
}

// Setup a deep node
func T(n *Node) *Tree {
	d := db.NewDB("test", db.MemDBBackendStr, "")
	t := NewTree(0, d)

	n.hashWithCount()
	t.root = n
	return t
}

// Convenience for simple printing of keys & tree structure
func P(n *Node) string {
	if n.height == 0 {
		return fmt.Sprintf("%v", b2i(n.key))
	} else {
		return fmt.Sprintf("(%v %v)", P(n.leftNode), P(n.rightNode))
	}
}

func randBytes(length int) []byte {
	key := make([]byte, length)
	// math.rand.Read always returns err=nil
	mrand.Read(key)
	return key
}

type traverser struct {
	first string
	last  string
	count int
}

func (t *traverser) view(key, value []byte) bool {
	if t.first == "" {
		t.first = string(key)
	}
	t.last = string(key)
	t.count += 1
	return false
}

func expectTraverse(t *testing.T, trav traverser, start, end string, count int) {
	if trav.first != start {
		t.Error("Bad start", start, trav.first)
	}
	if trav.last != end {
		t.Error("Bad end", end, trav.last)
	}
	if trav.count != count {
		t.Error("Bad count", count, trav.count)
	}
}

func testProof(t *testing.T, proof *KeyExistsProof, keyBytes, valueBytes, rootHashBytes []byte) {
	// Proof must verify.
	require.NoError(t, proof.Verify(keyBytes, valueBytes, rootHashBytes))

	// Write/Read then verify.
	proofBytes := proof.Bytes()
	proof2, err := ReadKeyExistsProof(proofBytes)
	require.Nil(t, err, "Failed to read KeyExistsProof from bytes: %v", err)
	require.NoError(t, proof2.Verify(keyBytes, valueBytes, proof.RootHash))

	// Random mutations must not verify
	for i := 0; i < 10; i++ {
		badProofBytes := MutateByteSlice(proofBytes)
		badProof, err := ReadKeyExistsProof(badProofBytes)
		// may be invalid... errors are okay
		if err == nil {
			assert.Error(t, badProof.Verify(keyBytes, valueBytes, rootHashBytes),
				"Proof was still valid after a random mutation:\n%X\n%X",
				proofBytes, badProofBytes)
		}
	}

	// targetted changes fails...
	proof.RootHash = MutateByteSlice(proof.RootHash)
	assert.Error(t, proof.Verify(keyBytes, valueBytes, rootHashBytes))
}

func BenchmarkImmutableAvlTreeMemDB(b *testing.B) {
	db := db.NewDB("test", db.MemDBBackendStr, "")
	benchmarkImmutableAvlTreeWithDB(b, db)
}

func benchmarkImmutableAvlTreeWithDB(b *testing.B, db db.DB) {
	defer db.Close()

	b.StopTimer()

	v := uint64(1)
	t := NewVersionedTree(100000, db)
	for i := 0; i < 1000000; i++ {
		t.Set(i2b(int(RandInt32())), nil)
		if i > 990000 && i%1000 == 999 {
			t.SaveVersion(v)
			v++
		}
	}
	b.ReportAllocs()
	t.SaveVersion(v)

	fmt.Println("ok, starting")

	runtime.GC()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		ri := i2b(int(RandInt32()))
		t.Set(ri, nil)
		t.Remove(ri)
		if i%100 == 99 {
			t.SaveVersion(v)
			v++
		}
	}
}
