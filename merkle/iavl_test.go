package merkle

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/db"

	"runtime"
	"testing"
)

func init() {
	// TODO: seed rand?
}

func randstr(length int) String {
	return String(RandStr(length))
}

func TestUnit(t *testing.T) {

	// Convenience for a new node
	N := func(l, r interface{}) *IAVLNode {
		var left, right *IAVLNode
		if _, ok := l.(*IAVLNode); ok {
			left = l.(*IAVLNode)
		} else {
			left = NewIAVLNode(Int32(l.(int)), nil)
		}
		if _, ok := r.(*IAVLNode); ok {
			right = r.(*IAVLNode)
		} else {
			right = NewIAVLNode(Int32(r.(int)), nil)
		}

		n := &IAVLNode{
			key:   right.lmd(nil).key,
			left:  left,
			right: right,
		}
		n.calcHeightAndSize(nil)
		n.Hash()
		return n
	}

	// Convenience for simple printing of keys & tree structure
	var P func(*IAVLNode) string
	P = func(n *IAVLNode) string {
		if n.height == 0 {
			return fmt.Sprintf("%v", n.key)
		} else {
			return fmt.Sprintf("(%v %v)", P(n.left), P(n.right))
		}
	}

	expectHash := func(n2 *IAVLNode, hashCount uint64) {
		// ensure number of new hash calculations is as expected.
		hash, count := n2.Hash()
		if count != hashCount {
			t.Fatalf("Expected %v new hashes, got %v", hashCount, count)
		}
		// nuke hashes and reconstruct hash, ensure it's the same.
		(&IAVLTree{root: n2}).Traverse(func(node Node) bool {
			node.(*IAVLNode).hash = nil
			return false
		})
		// ensure that the new hash after nuking is the same as the old.
		newHash, _ := n2.Hash()
		if bytes.Compare(hash, newHash) != 0 {
			t.Fatalf("Expected hash %v but got %v after nuking", hash, newHash)
		}
	}

	expectPut := func(n *IAVLNode, i int, repr string, hashCount uint64) {
		n2, updated := n.put(nil, Int32(i), nil)
		// ensure node was added & structure is as expected.
		if updated == true || P(n2) != repr {
			t.Fatalf("Adding %v to %v:\nExpected         %v\nUnexpectedly got %v updated:%v",
				i, P(n), repr, P(n2), updated)
		}
		// ensure hash calculation requirements
		expectHash(n2, hashCount)
	}

	expectRemove := func(n *IAVLNode, i int, repr string, hashCount uint64) {
		n2, _, value, err := n.remove(nil, Int32(i))
		// ensure node was added & structure is as expected.
		if value != nil || err != nil || P(n2) != repr {
			t.Fatalf("Removing %v from %v:\nExpected         %v\nUnexpectedly got %v value:%v err:%v",
				i, P(n), repr, P(n2), value, err)
		}
		// ensure hash calculation requirements
		expectHash(n2, hashCount)
	}

	//////// Test Put cases:

	// Case 1:
	n1 := N(4, 20)

	expectPut(n1, 8, "((4 8) 20)", 3)
	expectPut(n1, 25, "(4 (20 25))", 3)

	n2 := N(4, N(20, 25))

	expectPut(n2, 8, "((4 8) (20 25))", 3)
	expectPut(n2, 30, "((4 20) (25 30))", 4)

	n3 := N(N(1, 2), 6)

	expectPut(n3, 4, "((1 2) (4 6))", 4)
	expectPut(n3, 8, "((1 2) (6 8))", 3)

	n4 := N(N(1, 2), N(N(5, 6), N(7, 9)))

	expectPut(n4, 8, "(((1 2) (5 6)) ((7 8) 9))", 5)
	expectPut(n4, 10, "(((1 2) (5 6)) (7 (9 10)))", 5)

	//////// Test Remove cases:

	n10 := N(N(1, 2), 3)

	expectRemove(n10, 2, "(1 3)", 1)
	expectRemove(n10, 3, "(1 2)", 0)

	n11 := N(N(N(1, 2), 3), N(4, 5))

	expectRemove(n11, 4, "((1 2) (3 5))", 2)
	expectRemove(n11, 3, "((1 2) (4 5))", 1)

}

func TestIntegration(t *testing.T) {

	type record struct {
		key   String
		value String
	}

	records := make([]*record, 400)
	var tree *IAVLTree = NewIAVLTree(nil)
	var err error
	var val Value
	var updated bool

	randomRecord := func() *record {
		return &record{randstr(20), randstr(20)}
	}

	for i := range records {
		r := randomRecord()
		records[i] = r
		//t.Log("New record", r)
		//PrintIAVLNode(tree.root)
		updated = tree.Put(r.key, String(""))
		if updated {
			t.Error("should have not been updated")
		}
		updated = tree.Put(r.key, r.value)
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
		if val := tree.Get(r.key); !(val.(String)).Equals(r.value) {
			t.Error("wrong value")
		}
	}

	for i, x := range records {
		if val, err = tree.Remove(x.key); err != nil {
			t.Error(err)
		} else if !(val.(String)).Equals(x.value) {
			t.Error("wrong value")
		}
		for _, r := range records[i+1:] {
			if has := tree.Has(r.key); !has {
				t.Error("Missing key", r.key)
			}
			if has := tree.Has(randstr(12)); has {
				t.Error("Table has extra key")
			}
			if val := tree.Get(r.key); !(val.(String)).Equals(r.value) {
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
	records := make(map[String]String)
	for i := 0; i < 10000; i++ {
		records[String(randstr(20))] = String(randstr(20))
	}

	// Construct some tree and save it
	t1 := NewIAVLTree(db)
	for key, value := range records {
		t1.Put(key, value)
	}
	t1.Save()

	hash, _ := t1.Hash()

	// Load a tree
	t2 := NewIAVLTreeFromHash(db, hash)
	for key, value := range records {
		t2value := t2.Get(key)
		if !BinaryEqual(t2value, value) {
			t.Fatalf("Invalid value. Expected %v, got %v", value, t2value)
		}
	}
}

func BenchmarkHash(b *testing.B) {
	b.StopTimer()

	s := randstr(128)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		hasher := sha256.New()
		hasher.Write([]byte(s))
		hasher.Sum(nil)
	}
}

func BenchmarkImmutableAvlTree(b *testing.B) {
	b.StopTimer()

	type record struct {
		key   String
		value String
	}

	randomRecord := func() *record {
		return &record{randstr(32), randstr(32)}
	}

	t := NewIAVLTree(nil)
	for i := 0; i < 1000000; i++ {
		r := randomRecord()
		t.Put(r.key, r.value)
	}

	fmt.Println("ok, starting")

	runtime.GC()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		r := randomRecord()
		t.Put(r.key, r.value)
		t.Remove(r.key)
	}
}
