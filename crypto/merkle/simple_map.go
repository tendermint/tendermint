package merkle

import (
	cmn "github.com/tendermint/tendermint/libs/common"
)

// Merkle tree from a map.
// Leaves are `hash(key) | hash(value)`.
// Leaves are sorted before Merkle hashing.
type simpleMap struct {
	kvs    cmn.KVPairs
	sorted bool
}

func newSimpleMap() *simpleMap {
	return &simpleMap{
		kvs:    nil,
		sorted: false,
	}
}

// Set hashes the key and value and appends it to the kv pairs.
func (sm *simpleMap) Set(key string, value Hasher) {
	sm.sorted = false

	// The value is hashed, so you can
	// check for equality with a cached value (say)
	// and make a determination to fetch or not.
	vhash := value.Hash()

	sm.kvs = append(sm.kvs, cmn.KVPair{
		Key:   []byte(key),
		Value: vhash,
	})
}

// Hash computes the Merkle root hash of items sorted by key
// (UNSTABLE: and by value too if duplicate key).
func (sm *simpleMap) Hash() []byte {
	sm.Sort()
	return hashKVPairs(sm.kvs)
}

func (sm *simpleMap) Sort() {
	if sm.sorted {
		return
	}
	sm.kvs.Sort()
	sm.sorted = true
}

// Returns a copy of sorted KVPairs.
// NOTE these contain the hashed key and value.
func (sm *simpleMap) KVPairs() cmn.KVPairs {
	sm.Sort()
	kvs := make(cmn.KVPairs, len(sm.kvs))
	copy(kvs, sm.kvs)
	return kvs
}

//----------------------------------------

// A local extension to KVPair that can be hashed.
// Key and value are length prefixed and concatenated,
// then hashed.
type KVPair cmn.KVPair

func (kv KVPair) Bytes() []byte {
	val := make([]byte, len(kv.Key)+len(kv.Value))
	copy(val, kv.Key)
	copy(val[len(kv.Key):], kv.Value)
	return val
}

func hashKVPairs(kvs cmn.KVPairs) []byte {
	kvsH := make([][]byte, len(kvs))
	for i, kvp := range kvs {
		kvsH[i] = KVPair(kvp).Bytes()
	}
	return SimpleHashFromByteSlices(kvsH)
}
