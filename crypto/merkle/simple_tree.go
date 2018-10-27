package merkle

// simpleHashFromTwoHashes is the basic operation of the Merkle tree: Hash(left | right).
func simpleHashFromTwoHashes(left, right []byte) []byte {
	return innerHash(append(left, right...))
}

// SimpleHashFromByteSlices computes a Merkle tree where the leaves are the byte slice,
// in the provided order.
func SimpleHashFromByteSlices(items [][]byte) []byte {
	switch len(items) {
	case 0:
		return nil
	case 1:
		return leafHash(items[0])
	default:
		left := SimpleHashFromByteSlices(items[:(len(items)+1)/2])
		right := SimpleHashFromByteSlices(items[(len(items)+1)/2:])
		return simpleHashFromTwoHashes(left, right)
	}
}

// SimpleHashFromMap computes a Merkle tree from sorted map.
// Like calling SimpleHashFromHashers with
// `item = []byte(Hash(key) | Hash(value))`,
// sorted by `item`.
func SimpleHashFromMap(m map[string][]byte) []byte {
	sm := newSimpleMap()
	for k, v := range m {
		sm.Set(k, v)
	}
	return sm.Hash()
}
