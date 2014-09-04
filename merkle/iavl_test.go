package merkle

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/db"

	"runtime"
	"testing"
)

func init() {
	// TODO: seed rand?
}

func randstr(length int) string {
	return RandStr(length)
}

func TestUnit(t *testing.T) {

	// Convenience for a new node
	N := func(l, r interface{}) *IAVLNode {
		var left, right *IAVLNode
		if _, ok := l.(*IAVLNode); ok {
			left = l.(*IAVLNode)
		} else {
			left = NewIAVLNode([]byte{byte(l.(int))}, nil)
		}
		if _, ok := r.(*IAVLNode); ok {
			right = r.(*IAVLNode)
		} else {
			right = NewIAVLNode([]byte{byte(r.(int))}, nil)
		}

		n := &IAVLNode{
			key:   right.lmd(nil).key,
			left:  left,
			right: right,
		}
		n.calcHeightAndSize(nil)
		n.HashWithCount()
		return n
	}

	// Convenience for simple printing of keys & tree structure
	var P func(*IAVLNode) string
	P = func(n *IAVLNode) string {
		if n.height == 0 {
			return fmt.Sprintf("%v", n.key[0])
		} else {
			return fmt.Sprintf("(%v %v)", P(n.left), P(n.right))
		}
	}

	expectHash := func(n2 *IAVLNode, hashCount uint64) {
		// ensure number of new hash calculations is as expected.
		hash, count := n2.HashWithCount()
		if count != hashCount {
			t.Fatalf("Expected %v new hashes, got %v", hashCount, count)
		}
		// nuke hashes and reconstruct hash, ensure it's the same.
		n2.traverse(nil, func(node *IAVLNode) bool {
			node.hash = nil
			return false
		})
		// ensure that the new hash after nuking is the same as the old.
		newHash, _ := n2.HashWithCount()
		if bytes.Compare(hash, newHash) != 0 {
			t.Fatalf("Expected hash %v but got %v after nuking", hash, newHash)
		}
	}

	expectSet := func(n *IAVLNode, i int, repr string, hashCount uint64) {
		n2, updated := n.set(nil, []byte{byte(i)}, nil)
		// ensure node was added & structure is as expected.
		if updated == true || P(n2) != repr {
			t.Fatalf("Adding %v to %v:\nExpected         %v\nUnexpectedly got %v updated:%v",
				i, P(n), repr, P(n2), updated)
		}
		// ensure hash calculation requirements
		expectHash(n2, hashCount)
	}

	expectRemove := func(n *IAVLNode, i int, repr string, hashCount uint64) {
		n2, _, value, err := n.remove(nil, []byte{byte(i)})
		// ensure node was added & structure is as expected.
		if value != nil || err != nil || P(n2) != repr {
			t.Fatalf("Removing %v from %v:\nExpected         %v\nUnexpectedly got %v value:%v err:%v",
				i, P(n), repr, P(n2), value, err)
		}
		// ensure hash calculation requirements
		expectHash(n2, hashCount)
	}

	//////// Test Set cases:

	// Case 1:
	n1 := N(4, 20)

	expectSet(n1, 8, "((4 8) 20)", 3)
	expectSet(n1, 25, "(4 (20 25))", 3)

	n2 := N(4, N(20, 25))

	expectSet(n2, 8, "((4 8) (20 25))", 3)
	expectSet(n2, 30, "((4 20) (25 30))", 4)

	n3 := N(N(1, 2), 6)

	expectSet(n3, 4, "((1 2) (4 6))", 4)
	expectSet(n3, 8, "((1 2) (6 8))", 3)

	n4 := N(N(1, 2), N(N(5, 6), N(7, 9)))

	expectSet(n4, 8, "(((1 2) (5 6)) ((7 8) 9))", 5)
	expectSet(n4, 10, "(((1 2) (5 6)) (7 (9 10)))", 5)

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
		key   string
		value string
	}

	records := make([]*record, 400)
	var tree *IAVLTree = NewIAVLTree(nil)
	var err error
	var val []byte
	var updated bool

	randomRecord := func() *record {
		return &record{randstr(20), randstr(20)}
	}

	for i := range records {
		r := randomRecord()
		records[i] = r
		//t.Log("New record", r)
		//PrintIAVLNode(tree.root)
		updated = tree.Set([]byte(r.key), []byte(""))
		if updated {
			t.Error("should have not been updated")
		}
		updated = tree.Set([]byte(r.key), []byte(r.value))
		if !updated {
			t.Error("should have been updated")
		}
		if tree.Size() != uint64(i+1) {
			t.Error("size was wrong", tree.Size(), i+1)
		}
	}

	for _, r := range records {
		if has := tree.Has([]byte(r.key)); !has {
			t.Error("Missing key", r.key)
		}
		if has := tree.Has([]byte(randstr(12))); has {
			t.Error("Table has extra key")
		}
		if val := tree.Get([]byte(r.key)); string(val) != r.value {
			t.Error("wrong value")
		}
	}

	for i, x := range records {
		if val, err = tree.Remove([]byte(x.key)); err != nil {
			t.Error(err)
		} else if string(val) != x.value {
			t.Error("wrong value")
		}
		for _, r := range records[i+1:] {
			if has := tree.Has([]byte(r.key)); !has {
				t.Error("Missing key", r.key)
			}
			if has := tree.Has([]byte(randstr(12))); has {
				t.Error("Table has extra key")
			}
			val := tree.Get([]byte(r.key))
			if string(val) != r.value {
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
	t1 := NewIAVLTree(db)
	for key, value := range records {
		t1.Set([]byte(key), []byte(value))
	}
	t1.Save()

	hash, _ := t1.HashWithCount()

	// Load a tree
	t2 := NewIAVLTreeFromHash(db, hash)
	for key, value := range records {
		t2value := t2.Get([]byte(key))
		if string(t2value) != value {
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
		key   string
		value string
	}

	randomRecord := func() *record {
		return &record{randstr(32), randstr(32)}
	}

	t := NewIAVLTree(nil)
	for i := 0; i < 1000000; i++ {
		r := randomRecord()
		t.Set([]byte(r.key), []byte(r.value))
	}

	fmt.Println("ok, starting")

	runtime.GC()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		r := randomRecord()
		t.Set([]byte(r.key), []byte(r.value))
		t.Remove([]byte(r.key))
	}
}
