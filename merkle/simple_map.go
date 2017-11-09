package merkle

type SimpleMap struct {
	kvz KVPairs
}

func NewSimpleMap() *SimpleMap {
	return &SimpleMap{
		kvz: nil,
	}
}

func (sm *SimpleMap) Set(k string, o interface{}) {
	sm.kvz = append(sm.kvz, KVPair{Key: k, Value: o})
}

// Merkle root hash of items sorted by key.
// NOTE: Behavior is undefined when key is duplicate.
func (sm *SimpleMap) Hash() []byte {
	sm.kvz.Sort()
	kvPairsH := make([]Hashable, 0, len(sm.kvz))
	for _, kvp := range sm.kvz {
		kvPairsH = append(kvPairsH, kvp)
	}
	return SimpleHashFromHashables(kvPairsH)
}
