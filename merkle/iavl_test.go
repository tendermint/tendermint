package merkle

import "testing"

import (
    "fmt"
    "os"
    "bytes"
    "math/rand"
    "encoding/binary"
)


func init() {
    if urandom, err := os.Open("/dev/urandom"); err != nil {
        return
    } else {
        buf := make([]byte, 8)
        if _, err := urandom.Read(buf); err == nil {
            buf_reader := bytes.NewReader(buf)
            if seed, err := binary.ReadVarint(buf_reader); err == nil {
                rand.Seed(seed)
            }
        }
        urandom.Close()
    }
}

func randstr(length int) String {
    if urandom, err := os.Open("/dev/urandom"); err != nil {
        panic(err)
    } else {
        slice := make([]byte, length)
        if _, err := urandom.Read(slice); err != nil {
            panic(err)
        }
        urandom.Close()
        return String(slice)
    }
    panic("unreachable")
}

func TestImmutableAvlPutHasGetRemove(t *testing.T) {

    type record struct {
        key String
        value String
    }

    records := make([]*record, 400)
    var tree *IAVLNode
    var err error
    var val Value
    var updated bool

    ranrec := func() *record {
        return &record{ randstr(20), randstr(20) }
    }

    for i := range records {
        r := ranrec()
        records[i] = r
        tree, updated = tree.Put(r.key, String(""))
        if updated {
            t.Error("should have not been updated")
        }
        tree, updated = tree.Put(r.key, r.value)
        if !updated {
            t.Error("should have been updated")
        }
        if tree.Size() != (i+1) {
            t.Error("size was wrong", tree.Size(), i+1)
        }
    }

    for _, r := range records {
        if has := tree.Has(r.key); !has {
            t.Error("Missing key")
        }
        if has := tree.Has(randstr(12)); has {
            t.Error("Table has extra key")
        }
        if val, err := tree.Get(r.key); err != nil {
            t.Error(err, val.(String), r.value)
        } else if !(val.(String)).Equals(r.value) {
            t.Error("wrong value")
        }
    }

    for i, x := range records {
        if tree, val, err = tree.Remove(x.key); err != nil {
            t.Error(err)
        } else if !(val.(String)).Equals(x.value) {
            t.Error("wrong value")
        }
        for _, r := range records[i+1:] {
            if has := tree.Has(r.key); !has {
                t.Error("Missing key")
            }
            if has := tree.Has(randstr(12)); has {
                t.Error("Table has extra key")
            }
            if val, err := tree.Get(r.key); err != nil {
                t.Error(err)
            } else if !(val.(String)).Equals(r.value) {
                t.Error("wrong value")
            }
        }
        if tree.Size() != (len(records) - (i+1)) {
            t.Error("size was wrong", tree.Size(), (len(records) - (i+1)))
        }
    }
}


func BenchmarkImmutableAvlTree(b *testing.B) {
    b.StopTimer()

    type record struct {
        key String
        value String
    }

    records := make([]*record, 100)

    ranrec := func() *record {
        return &record{ randstr(20), randstr(20) }
    }

    for i := range records {
        records[i] = ranrec()
    }

    b.StartTimer()
    for i := 0; i < b.N; i++ {
        t := NewIAVLTree()
        for _, r := range records {
            t.Put(r.key, r.value)
        }
        for _, r := range records {
            t.Remove(r.key)
        }
    }
}


func TestTraversals(t *testing.T) {
    var data []int = []int{
        1, 5, 7, 9, 12, 13, 17, 18, 19, 20,
    }
    var order []int = []int{
        6, 1, 8, 2, 4 , 9 , 5 , 7 , 0 , 3 ,
    }

    test := func(T Tree) {
        t.Logf("%T", T)
        for j := range order {
            if err := T.Put(Int(data[order[j]]), Int(order[j])); err != nil {
                t.Error(err)
            }
        }

        j := 0
        for
          tn, next := Iterator(T.Root())();
          next != nil;
          tn, next = next () {
            if int(tn.Key().(Int)) != data[j] {
                t.Error("key in wrong spot in-order")
            }
            j += 1
        }
    }
    test(NewIAVLTree())
}

// from http://stackoverflow.com/questions/3955680/how-to-check-if-my-avl-tree-implementation-is-correct
func TestGriffin(t *testing.T) {

    // Convenience for a new node
    N := func(l *IAVLNode, i int, r *IAVLNode) *IAVLNode {
        n := &IAVLNode{Int32(i), nil, -1, nil, l, r}
        n.calc_height()
        n.Hash()
        return n
    }

    // Convenience for simple printing of keys & tree structure
    var P func(*IAVLNode) string
    P = func(n *IAVLNode) string {
        if n.left == nil && n.right == nil {
            return fmt.Sprintf("%v", n.key)
        } else if n.left == nil {
            return fmt.Sprintf("(- %v %v)", n.key, P(n.right))
        } else if n.right == nil {
            return fmt.Sprintf("(%v %v -)", P(n.left), n.key)
        } else {
            return fmt.Sprintf("(%v %v %v)", P(n.left), n.key, P(n.right))
        }
    }

    expectHash := func(n2 *IAVLNode, hashCount int) {
        // ensure number of new hash calculations is as expected.
        hash, count := n2.Hash()
        if count != hashCount {
            t.Fatalf("Expected %v new hashes, got %v", hashCount, count)
        }
        // nuke hashes and reconstruct hash, ensure it's the same.
        var node Node
        for itr:=Iterator(n2); itr!=nil; {
            node, itr = itr()
            if node != nil {
                node.(*IAVLNode).hash = nil
            }
        }
        // ensure that the new hash after nuking is the same as the old.
        newHash, _ := n2.Hash()
        if bytes.Compare(hash, newHash) != 0 {
            t.Fatalf("Expected hash %v but got %v after nuking", hash, newHash)
        }
    }

    expectPut := func(n *IAVLNode, i int, repr string, hashCount int) {
        n2, updated := n.Put(Int32(i), nil)
        // ensure node was added & structure is as expected.
        if updated == true || P(n2) != repr {
            t.Fatalf("Adding %v to %v:\nExpected         %v\nUnexpectedly got %v updated:%v",
                i, P(n), repr, P(n2), updated)
        }
        // ensure hash calculation requirements
        expectHash(n2, hashCount)
    }

    expectRemove := func(n *IAVLNode, i int, repr string, hashCount int) {
        n2, value, err := n.Remove(Int32(i))
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
    n1 := N(N(nil, 4, nil), 20, nil)
    if P(n1) != "(4 20 -)" { t.Fatalf("Got %v", P(n1)) }

    expectPut(n1, 15, "(4 15 20)", 3)
    expectPut(n1, 8, "(4 8 20)", 3)

    // Case 2:
    n2 := N(N(N(nil, 3, nil), 4, N(nil, 9, nil)), 20, N(nil, 26, nil))
    if P(n2) != "((3 4 9) 20 26)" { t.Fatalf("Got %v", P(n2)) }

    expectPut(n2, 15, "((3 4 -) 9 (15 20 26))", 4)
    expectPut(n2, 8, "((3 4 8) 9 (- 20 26))", 4)

    // Case 2:
    n3 := N(N(N(N(nil, 2, nil), 3, nil), 4, N(N(nil, 7, nil), 9, N(nil, 11, nil))),
        20, N(N(nil, 21, nil), 26, N(nil, 30, nil)))
    if P(n3) != "(((2 3 -) 4 (7 9 11)) 20 (21 26 30))" { t.Fatalf("Got %v", P(n3)) }

    expectPut(n3, 15, "(((2 3 -) 4 7) 9 ((- 11 15) 20 (21 26 30)))", 5)
    expectPut(n3, 8, "(((2 3 -) 4 (- 7 8)) 9 (11 20 (21 26 30)))", 5)


    //////// Test Remove cases:

    // Case 4:
    n4 := N(N(nil, 1, nil), 2, N(N(nil, 3, nil), 4, N(nil, 5, nil)))
    if P(n4) != "(1 2 (3 4 5))" { t.Fatalf("Got %v", P(n4)) }

    expectRemove(n4, 1, "((- 2 3) 4 5)", 2)

    // Case 5:
    n5 := N(N(N(nil, 1, nil), 2, N(N(nil, 3, nil), 4, N(nil, 5, nil))), 6,
            N(N(N(nil, 7, nil), 8, nil), 9, N(N(nil, 10, nil), 11, N(nil, 12, N(nil, 13, nil)))))
    if P(n5) != "((1 2 (3 4 5)) 6 ((7 8 -) 9 (10 11 (- 12 13))))" { t.Fatalf("Got %v", P(n5)) }

    expectRemove(n5, 1, "(((- 2 3) 4 5) 6 ((7 8 -) 9 (10 11 (- 12 13))))", 3)

    // Case 6:
    n6 := N(N(N(nil, 1, nil), 2, N(nil, 3, N(nil, 4, nil))), 5,
            N(N(N(nil, 6, nil), 7, nil), 8, N(N(nil, 9, nil), 10, N(nil, 11, N(nil, 12, nil)))))
    if P(n6) != "((1 2 (- 3 4)) 5 ((6 7 -) 8 (9 10 (- 11 12))))" { t.Fatalf("Got %v", P(n6)) }

    expectRemove(n6, 1, "(((2 3 4) 5 (6 7 -)) 8 (9 10 (- 11 12)))", 4)

}
