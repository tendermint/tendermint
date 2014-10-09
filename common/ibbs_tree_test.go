package common

import (
	"fmt"
	"runtime"
	"testing"
)

func init() {
	// TODO: seed rand?
}

func (self *IBBSTree) lmd() *IBBSTree {
	if self.height == 0 {
		return self
	}
	return self.left.lmd()
}

func TestUnit(t *testing.T) {

	// Convenience for a new node
	N := func(l, r interface{}) *IBBSTree {
		var left, right *IBBSTree
		if _, ok := l.(*IBBSTree); ok {
			left = l.(*IBBSTree)
		} else {
			left = NewIBBSTreeNode(uint64(l.(int)), nil)
		}
		if _, ok := r.(*IBBSTree); ok {
			right = r.(*IBBSTree)
		} else {
			right = NewIBBSTreeNode(uint64(r.(int)), nil)
		}

		n := &IBBSTree{
			key:   right.lmd().key,
			left:  left,
			right: right,
		}
		n.calcHeightAndSize()
		return n
	}

	// Convenience for simple printing of keys & tree structure
	var P func(*IBBSTree) string
	P = func(n *IBBSTree) string {
		if n.height == 0 {
			return fmt.Sprintf("%v", n.key)
		} else {
			return fmt.Sprintf("(%v %v)", P(n.left), P(n.right))
		}
	}

	expectSet := func(n *IBBSTree, i uint64, repr string) {
		n2, updated := n.Set(i, nil)
		// ensure node was added & structure is as expected.
		if updated == true || P(n2) != repr {
			t.Fatalf("Adding %v to %v:\nExpected         %v\nUnexpectedly got %v updated:%v",
				i, P(n), repr, P(n2), updated)
		}
	}

	expectRemove := func(n *IBBSTree, i uint64, repr string) {
		n2, value, removed := n.Remove(i)
		// ensure node was removed & structure is as expected.
		if value != nil || P(n2) != repr || !removed {
			t.Fatalf("Removing %v from %v:\nExpected         %v\nUnexpectedly got %v value:%v err:%v",
				i, P(n), repr, P(n2), value, removed)
		}
	}

	//////// Test Set cases:

	// Case 1:
	n1 := N(4, 20)

	expectSet(n1, 8, "((4 8) 20)")
	expectSet(n1, 25, "(4 (20 25))")

	n2 := N(4, N(20, 25))

	expectSet(n2, 8, "((4 8) (20 25))")
	expectSet(n2, 30, "((4 20) (25 30))")

	n3 := N(N(1, 2), 6)

	expectSet(n3, 4, "((1 2) (4 6))")
	expectSet(n3, 8, "((1 2) (6 8))")

	n4 := N(N(1, 2), N(N(5, 6), N(7, 9)))

	expectSet(n4, 8, "(((1 2) (5 6)) ((7 8) 9))")
	expectSet(n4, 10, "(((1 2) (5 6)) (7 (9 10)))")

	//////// Test Remove cases:

	n10 := N(N(1, 2), 3)

	expectRemove(n10, 2, "(1 3)")
	expectRemove(n10, 3, "(1 2)")

	n11 := N(N(N(1, 2), 3), N(4, 5))

	expectRemove(n11, 4, "((1 2) (3 5))")
	expectRemove(n11, 3, "((1 2) (4 5))")

}

func TestIntegration(t *testing.T) {

	type record struct {
		key   uint64
		value string
	}

	records := make([]*record, 400)
	var tree *IBBSTree = NewIBBSTree()
	var val interface{}
	var removed bool
	var updated bool

	randomRecord := func() *record {
		return &record{RandUInt64(), RandStr(20)}
	}

	for i := range records {
		r := randomRecord()
		records[i] = r
		//t.Log("New record", r)
		//PrintIBBSTree(tree.root)
		tree, updated = tree.Set(r.key, "")
		if updated {
			t.Error("should have not been updated")
		}
		tree, updated = tree.Set(r.key, r.value)
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
		if val := tree.Get(r.key); val.(string) != r.value {
			t.Error("wrong value")
		}
	}

	for i, x := range records {
		if tree, val, removed = tree.Remove(x.key); !removed {
			t.Error("not removed")
		} else if val.(string) != x.value {
			t.Error("wrong value")
		}
		for _, r := range records[i+1:] {
			if has := tree.Has(r.key); !has {
				t.Error("Missing key", r.key)
			}
			val := tree.Get(r.key)
			if val.(string) != r.value {
				t.Error("wrong value")
			}
		}
		if tree.Size() != uint64(len(records)-(i+1)) {
			t.Error("size was wrong", tree.Size(), (len(records) - (i + 1)))
		}
	}
}

func BenchmarkIBBSTree(b *testing.B) {
	b.StopTimer()

	type record struct {
		key   uint64
		value interface{}
	}

	randomRecord := func() *record {
		return &record{RandUInt64(), RandUInt64()}
	}

	t := NewIBBSTree()
	for i := 0; i < 1000000; i++ {
		r := randomRecord()
		t, _ = t.Set(r.key, r.value)
	}

	fmt.Println("ok, starting")

	runtime.GC()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		r := randomRecord()
		t, _ = t.Set(r.key, r.value)
		t, _, _ = t.Remove(r.key)
	}
}
