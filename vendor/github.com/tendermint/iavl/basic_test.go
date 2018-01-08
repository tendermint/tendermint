package iavl

import (
	"bytes"
	mrand "math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tmlibs/db"
)

func TestBasic(t *testing.T) {
	var tree *Tree = NewTree(0, nil)
	up := tree.Set([]byte("1"), []byte("one"))
	if up {
		t.Error("Did not expect an update (should have been create)")
	}
	up = tree.Set([]byte("2"), []byte("two"))
	if up {
		t.Error("Did not expect an update (should have been create)")
	}
	up = tree.Set([]byte("2"), []byte("TWO"))
	if !up {
		t.Error("Expected an update")
	}
	up = tree.Set([]byte("5"), []byte("five"))
	if up {
		t.Error("Did not expect an update (should have been create)")
	}

	// Test 0x00
	{
		idx, val := tree.Get([]byte{0x00})
		if val != nil {
			t.Errorf("Expected no value to exist")
		}
		if idx != 0 {
			t.Errorf("Unexpected idx %x", idx)
		}
		if string(val) != "" {
			t.Errorf("Unexpected value %v", string(val))
		}
	}

	// Test "1"
	{
		idx, val := tree.Get([]byte("1"))
		if val == nil {
			t.Errorf("Expected value to exist")
		}
		if idx != 0 {
			t.Errorf("Unexpected idx %x", idx)
		}
		if string(val) != "one" {
			t.Errorf("Unexpected value %v", string(val))
		}
	}

	// Test "2"
	{
		idx, val := tree.Get([]byte("2"))
		if val == nil {
			t.Errorf("Expected value to exist")
		}
		if idx != 1 {
			t.Errorf("Unexpected idx %x", idx)
		}
		if string(val) != "TWO" {
			t.Errorf("Unexpected value %v", string(val))
		}
	}

	// Test "4"
	{
		idx, val := tree.Get([]byte("4"))
		if val != nil {
			t.Errorf("Expected no value to exist")
		}
		if idx != 2 {
			t.Errorf("Unexpected idx %x", idx)
		}
		if string(val) != "" {
			t.Errorf("Unexpected value %v", string(val))
		}
	}

	// Test "6"
	{
		idx, val := tree.Get([]byte("6"))
		if val != nil {
			t.Errorf("Expected no value to exist")
		}
		if idx != 3 {
			t.Errorf("Unexpected idx %x", idx)
		}
		if string(val) != "" {
			t.Errorf("Unexpected value %v", string(val))
		}
	}
}

func TestUnit(t *testing.T) {

	expectHash := func(tree *Tree, hashCount int) {
		// ensure number of new hash calculations is as expected.
		hash, count := tree.hashWithCount()
		if count != hashCount {
			t.Fatalf("Expected %v new hashes, got %v", hashCount, count)
		}
		// nuke hashes and reconstruct hash, ensure it's the same.
		tree.root.traverse(tree, true, func(node *Node) bool {
			node.hash = nil
			return false
		})
		// ensure that the new hash after nuking is the same as the old.
		newHash, _ := tree.hashWithCount()
		if !bytes.Equal(hash, newHash) {
			t.Fatalf("Expected hash %v but got %v after nuking", hash, newHash)
		}
	}

	expectSet := func(tree *Tree, i int, repr string, hashCount int) {
		origNode := tree.root
		updated := tree.Set(i2b(i), []byte{})
		// ensure node was added & structure is as expected.
		if updated || P(tree.root) != repr {
			t.Fatalf("Adding %v to %v:\nExpected         %v\nUnexpectedly got %v updated:%v",
				i, P(origNode), repr, P(tree.root), updated)
		}
		// ensure hash calculation requirements
		expectHash(tree, hashCount)
		tree.root = origNode
	}

	expectRemove := func(tree *Tree, i int, repr string, hashCount int) {
		origNode := tree.root
		value, removed := tree.Remove(i2b(i))
		// ensure node was added & structure is as expected.
		if len(value) != 0 || !removed || P(tree.root) != repr {
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

func TestRemove(t *testing.T) {
	size := 10000
	keyLen, dataLen := 16, 40

	d := db.NewDB("test", "memdb", "")
	defer d.Close()
	t1 := NewVersionedTree(size, d)

	// insert a bunch of random nodes
	keys := make([][]byte, size)
	l := int32(len(keys))
	for i := 0; i < size; i++ {
		key := randBytes(keyLen)
		t1.Set(key, randBytes(dataLen))
		keys[i] = key
	}

	for i := 0; i < 10; i++ {
		step := 50 * i
		// remove a bunch of existing keys (may have been deleted twice)
		for j := 0; j < step; j++ {
			key := keys[mrand.Int31n(l)]
			t1.Remove(key)
		}
		t1.SaveVersion(uint64(i))
	}
}

func TestIntegration(t *testing.T) {

	type record struct {
		key   string
		value string
	}

	records := make([]*record, 400)
	var tree *Tree = NewTree(0, nil)

	randomRecord := func() *record {
		return &record{randstr(20), randstr(20)}
	}

	for i := range records {
		r := randomRecord()
		records[i] = r
		updated := tree.Set([]byte(r.key), []byte{})
		if updated {
			t.Error("should have not been updated")
		}
		updated = tree.Set([]byte(r.key), []byte(r.value))
		if !updated {
			t.Error("should have been updated")
		}
		if tree.Size() != i+1 {
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
		if _, val := tree.Get([]byte(r.key)); string(val) != string(r.value) {
			t.Error("wrong value")
		}
	}

	for i, x := range records {
		if val, removed := tree.Remove([]byte(x.key)); !removed {
			t.Error("Wasn't removed")
		} else if string(val) != string(x.value) {
			t.Error("Wrong value")
		}
		for _, r := range records[i+1:] {
			if has := tree.Has([]byte(r.key)); !has {
				t.Error("Missing key", r.key)
			}
			if has := tree.Has([]byte(randstr(12))); has {
				t.Error("Table has extra key")
			}
			_, val := tree.Get([]byte(r.key))
			if string(val) != string(r.value) {
				t.Error("wrong value")
			}
		}
		if tree.Size() != len(records)-(i+1) {
			t.Error("size was wrong", tree.Size(), (len(records) - (i + 1)))
		}
	}
}

func TestIterateRange(t *testing.T) {
	type record struct {
		key   string
		value string
	}

	records := []record{
		{"abc", "123"},
		{"low", "high"},
		{"fan", "456"},
		{"foo", "a"},
		{"foobaz", "c"},
		{"good", "bye"},
		{"foobang", "d"},
		{"foobar", "b"},
		{"food", "e"},
		{"foml", "f"},
	}
	keys := make([]string, len(records))
	for i, r := range records {
		keys[i] = r.key
	}
	sort.Strings(keys)

	var tree *Tree = NewTree(0, nil)

	// insert all the data
	for _, r := range records {
		updated := tree.Set([]byte(r.key), []byte(r.value))
		if updated {
			t.Error("should have not been updated")
		}
	}

	// test traversing the whole node works... in order
	viewed := []string{}
	tree.Iterate(func(key []byte, value []byte) bool {
		viewed = append(viewed, string(key))
		return false
	})
	if len(viewed) != len(keys) {
		t.Error("not the same number of keys as expected")
	}
	for i, v := range viewed {
		if v != keys[i] {
			t.Error("Keys out of order", v, keys[i])
		}
	}

	trav := traverser{}
	tree.IterateRange([]byte("foo"), []byte("goo"), true, trav.view)
	expectTraverse(t, trav, "foo", "food", 5)

	trav = traverser{}
	tree.IterateRange(nil, []byte("flap"), true, trav.view)
	expectTraverse(t, trav, "abc", "fan", 2)

	trav = traverser{}
	tree.IterateRange([]byte("foob"), nil, true, trav.view)
	expectTraverse(t, trav, "foobang", "low", 6)

	trav = traverser{}
	tree.IterateRange([]byte("very"), nil, true, trav.view)
	expectTraverse(t, trav, "", "", 0)

	// make sure it doesn't include end
	trav = traverser{}
	tree.IterateRange([]byte("fooba"), []byte("food"), true, trav.view)
	expectTraverse(t, trav, "foobang", "foobaz", 3)

	// make sure backwards also works... (doesn't include end)
	trav = traverser{}
	tree.IterateRange([]byte("fooba"), []byte("food"), false, trav.view)
	expectTraverse(t, trav, "foobaz", "foobang", 3)

	// make sure backwards also works...
	trav = traverser{}
	tree.IterateRange([]byte("g"), nil, false, trav.view)
	expectTraverse(t, trav, "low", "good", 2)
}

func TestPersistence(t *testing.T) {
	db := db.NewMemDB()

	// Create some random key value pairs
	records := make(map[string]string)
	for i := 0; i < 10000; i++ {
		records[randstr(20)] = randstr(20)
	}

	// Construct some tree and save it
	t1 := NewVersionedTree(0, db)
	for key, value := range records {
		t1.Set([]byte(key), []byte(value))
	}
	t1.SaveVersion(1)

	// Load a tree
	t2 := NewVersionedTree(0, db)
	t2.Load()
	for key, value := range records {
		_, t2value := t2.Get([]byte(key))
		if string(t2value) != value {
			t.Fatalf("Invalid value. Expected %v, got %v", value, t2value)
		}
	}
}

func TestProof(t *testing.T) {
	t.Skipf("This test has a race condition causing it to occasionally panic.")

	// Construct some random tree
	db := db.NewMemDB()
	var tree *VersionedTree = NewVersionedTree(100, db)
	for i := 0; i < 1000; i++ {
		key, value := randstr(20), randstr(20)
		tree.Set([]byte(key), []byte(value))
	}

	// Persist the items so far
	tree.SaveVersion(1)

	// Add more items so it's not all persisted
	for i := 0; i < 100; i++ {
		key, value := randstr(20), randstr(20)
		tree.Set([]byte(key), []byte(value))
	}

	// Now for each item, construct a proof and verify
	tree.Iterate(func(key []byte, value []byte) bool {
		value2, proof, err := tree.GetWithProof(key)
		assert.NoError(t, err)
		assert.Equal(t, value, value2)
		if assert.NotNil(t, proof) {
			testProof(t, proof.(*KeyExistsProof), key, value, tree.Hash())
		}
		return false
	})
}

func TestTreeProof(t *testing.T) {
	db := db.NewMemDB()
	var tree *Tree = NewTree(100, db)

	// should get false for proof with nil root
	_, _, err := tree.GetWithProof([]byte("foo"))
	assert.Error(t, err)

	// insert lots of info and store the bytes
	keys := make([][]byte, 200)
	for i := 0; i < 200; i++ {
		key, value := randstr(20), randstr(200)
		tree.Set([]byte(key), []byte(value))
		keys[i] = []byte(key)
	}

	// query random key fails
	_, _, err = tree.GetWithProof([]byte("foo"))
	assert.NoError(t, err)

	// valid proof for real keys
	root := tree.Hash()
	for _, key := range keys {
		value, proof, err := tree.GetWithProof(key)
		if assert.NoError(t, err) {
			require.Nil(t, err, "Failed to read proof from bytes: %v", err)
			assert.NoError(t, proof.Verify(key, value, root))
		}
	}
}
