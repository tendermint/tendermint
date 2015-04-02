package merkle

import (
	"bytes"
	"fmt"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/db"

	"runtime"
	"testing"
)

func randstr(length int) string {
	return RandStr(length)
}

// Convenience for a new node
func N(l, r interface{}) *IAVLNode {
	var left, right *IAVLNode
	if _, ok := l.(*IAVLNode); ok {
		left = l.(*IAVLNode)
	} else {
		left = NewIAVLNode(l, "")
	}
	if _, ok := r.(*IAVLNode); ok {
		right = r.(*IAVLNode)
	} else {
		right = NewIAVLNode(r, "")
	}

	n := &IAVLNode{
		key:       right.lmd(nil).key,
		value:     "",
		leftNode:  left,
		rightNode: right,
	}
	n.calcHeightAndSize(nil)
	return n
}

// Setup a deep node
func T(n *IAVLNode) *IAVLTree {
	t := NewIAVLTree(binary.BasicCodec, binary.BasicCodec, 0, nil)
	n.hashWithCount(t)
	t.root = n
	return t
}

// Convenience for simple printing of keys & tree structure
func P(n *IAVLNode) string {
	if n.height == 0 {
		return fmt.Sprintf("%v", n.key)
	} else {
		return fmt.Sprintf("(%v %v)", P(n.leftNode), P(n.rightNode))
	}
}

func TestUnit(t *testing.T) {

	expectHash := func(tree *IAVLTree, hashCount uint64) {
		// ensure number of new hash calculations is as expected.
		hash, count := tree.HashWithCount()
		if count != hashCount {
			t.Fatalf("Expected %v new hashes, got %v", hashCount, count)
		}
		// nuke hashes and reconstruct hash, ensure it's the same.
		tree.root.traverse(tree, func(node *IAVLNode) bool {
			node.hash = nil
			return false
		})
		// ensure that the new hash after nuking is the same as the old.
		newHash, _ := tree.HashWithCount()
		if bytes.Compare(hash, newHash) != 0 {
			t.Fatalf("Expected hash %v but got %v after nuking", hash, newHash)
		}
	}

	expectSet := func(tree *IAVLTree, i int, repr string, hashCount uint64) {
		origNode := tree.root
		updated := tree.Set(i, "")
		// ensure node was added & structure is as expected.
		if updated == true || P(tree.root) != repr {
			t.Fatalf("Adding %v to %v:\nExpected         %v\nUnexpectedly got %v updated:%v",
				i, P(origNode), repr, P(tree.root), updated)
		}
		// ensure hash calculation requirements
		expectHash(tree, hashCount)
		tree.root = origNode
	}

	expectRemove := func(tree *IAVLTree, i int, repr string, hashCount uint64) {
		origNode := tree.root
		value, removed := tree.Remove(i)
		// ensure node was added & structure is as expected.
		if value != "" || !removed || P(tree.root) != repr {
			t.Fatalf("Removing %v from %v:\nExpected         %v\nUnexpectedly got %v value:%v removed:%v",
				i, P(origNode), repr, P(tree.root), value, removed)
		}
		// ensure hash calculation requirements
		expectHash(tree, hashCount)
		tree.root = origNode
	}

	//////// Test Set cases:

	// Case 1:
	t1 := T(N(4, 20))

	expectSet(t1, 8, "((4 8) 20)", 3)
	expectSet(t1, 25, "(4 (20 25))", 3)

	t2 := T(N(4, N(20, 25)))

	expectSet(t2, 8, "((4 8) (20 25))", 3)
	expectSet(t2, 30, "((4 20) (25 30))", 4)

	t3 := T(N(N(1, 2), 6))

	expectSet(t3, 4, "((1 2) (4 6))", 4)
	expectSet(t3, 8, "((1 2) (6 8))", 3)

	t4 := T(N(N(1, 2), N(N(5, 6), N(7, 9))))

	expectSet(t4, 8, "(((1 2) (5 6)) ((7 8) 9))", 5)
	expectSet(t4, 10, "(((1 2) (5 6)) (7 (9 10)))", 5)

	//////// Test Remove cases:

	t10 := T(N(N(1, 2), 3))

	expectRemove(t10, 2, "(1 3)", 1)
	expectRemove(t10, 3, "(1 2)", 0)

	t11 := T(N(N(N(1, 2), 3), N(4, 5)))

	expectRemove(t11, 4, "((1 2) (3 5))", 2)
	expectRemove(t11, 3, "((1 2) (4 5))", 1)

}

func TestIntegration(t *testing.T) {

	type record struct {
		key   string
		value string
	}

	records := make([]*record, 400)
	var tree *IAVLTree = NewIAVLTree(binary.BasicCodec, binary.BasicCodec, 0, nil)

	randomRecord := func() *record {
		return &record{randstr(20), randstr(20)}
	}

	for i := range records {
		r := randomRecord()
		records[i] = r
		//t.Log("New record", r)
		//PrintIAVLNode(tree.root)
		updated := tree.Set(r.key, "")
		if updated {
			t.Error("should have not been updated")
		}
		updated = tree.Set(r.key, r.value)
		if !updated {
			t.Error("should have been updated")
		}
		if tree.Size() != uint64(i+1) {
			t.Error("size was wrong", tree.Size(), i+1)
		}
	}

	for _, r := range records {
		if has := tree.Has(r.key); !has {
			t.Error("Missing key", r.key)
		}
		if has := tree.Has(randstr(12)); has {
			t.Error("Table has extra key")
		}
		if _, val := tree.Get(r.key); val.(string) != r.value {
			t.Error("wrong value")
		}
	}

	for i, x := range records {
		if val, removed := tree.Remove(x.key); !removed {
			t.Error("Wasn't removed")
		} else if val != x.value {
			t.Error("Wrong value")
		}
		for _, r := range records[i+1:] {
			if has := tree.Has(r.key); !has {
				t.Error("Missing key", r.key)
			}
			if has := tree.Has(randstr(12)); has {
				t.Error("Table has extra key")
			}
			_, val := tree.Get(r.key)
			if val != r.value {
				t.Error("wrong value")
			}
		}
		if tree.Size() != uint64(len(records)-(i+1)) {
			t.Error("size was wrong", tree.Size(), (len(records) - (i + 1)))
		}
	}
}

func TestPersistence(t *testing.T) {
	db := db.NewMemDB()

	// Create some random key value pairs
	records := make(map[string]string)
	for i := 0; i < 10000; i++ {
		records[randstr(20)] = randstr(20)
	}

	// Construct some tree and save it
	t1 := NewIAVLTree(binary.BasicCodec, binary.BasicCodec, 0, db)
	for key, value := range records {
		t1.Set(key, value)
	}
	t1.Save()

	hash, _ := t1.HashWithCount()

	// Load a tree
	t2 := NewIAVLTree(binary.BasicCodec, binary.BasicCodec, 0, db)
	t2.Load(hash)
	for key, value := range records {
		_, t2value := t2.Get(key)
		if t2value != value {
			t.Fatalf("Invalid value. Expected %v, got %v", value, t2value)
		}
	}
}

func BenchmarkImmutableAvlTree(b *testing.B) {
	b.StopTimer()

	t := NewIAVLTree(binary.BasicCodec, binary.BasicCodec, 0, nil)
	for i := 0; i < 1000000; i++ {
		t.Set(RandUint64(), "")
	}

	fmt.Println("ok, starting")

	runtime.GC()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		ri := RandUint64()
		t.Set(ri, "")
		t.Remove(ri)
	}
}
