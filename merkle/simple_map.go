package merkle

import (
	"github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
	"golang.org/x/crypto/ripemd160"
)

type SimpleMap struct {
	kvs    cmn.KVPairs
	sorted bool
}

func NewSimpleMap() *SimpleMap {
	return &SimpleMap{
		kvs:    nil,
		sorted: false,
	}
}

func (sm *SimpleMap) Set(key string, value interface{}) {
	sm.sorted = false

	// Is value Hashable?
	var vBytes []byte
	if hashable, ok := value.(Hashable); ok {
		vBytes = hashable.Hash()
	} else {
		vBytes, _ = wire.MarshalBinary(value)
	}

	sm.kvs = append(sm.kvs, cmn.KVPair{
		Key:   []byte(key),
		Value: vBytes,
	})
}

// Merkle root hash of items sorted by key.
// NOTE: Behavior is undefined when key is duplicate.
func (sm *SimpleMap) Hash() []byte {
	sm.Sort()
	return hashKVPairs(sm.kvs)
}

func (sm *SimpleMap) Sort() {
	if sm.sorted {
		return
	}
	sm.kvs.Sort()
	sm.sorted = true
}

// Returns a copy of sorted KVPairs.
// CONTRACT: The returned slice must not be mutated.
func (sm *SimpleMap) KVPairs() cmn.KVPairs {
	sm.Sort()
	kvs := make(cmn.KVPairs, len(sm.kvs))
	copy(kvs, sm.kvs)
	return kvs
}

//----------------------------------------

// A local extension to KVPair that can be hashed.
type kvPair cmn.KVPair

func (kv kvPair) Hash() []byte {
	hasher := ripemd160.New()
	err := wire.EncodeByteSlice(hasher, kv.Key)
	if err != nil {
		panic(err)
	}
	err = wire.EncodeByteSlice(hasher, kv.Value)
	if err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}

func hashKVPairs(kvs cmn.KVPairs) []byte {
	kvsH := make([]Hashable, 0, len(kvs))
	for _, kvp := range kvs {
		kvsH = append(kvsH, kvPair(kvp))
	}
	return SimpleHashFromHashables(kvsH)
}
