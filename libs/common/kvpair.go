package common

import (
	"bytes"
	"sort"
)

//----------------------------------------
// KVPair

/*
Defined in types.proto

type KVPair struct {
	Key   []byte
	Value []byte
}
*/

type KVPairs []KVPair

// Sorting
func (kvs KVPairs) Len() int { return len(kvs) }
func (kvs KVPairs) Less(i, j int) bool {
	switch bytes.Compare(kvs[i].Key, kvs[j].Key) {
	case -1:
		return true
	case 0:
		return bytes.Compare(kvs[i].Value, kvs[j].Value) < 0
	case 1:
		return false
	default:
		panic("invalid comparison result")
	}
}
func (kvs KVPairs) Swap(i, j int) { kvs[i], kvs[j] = kvs[j], kvs[i] }
func (kvs KVPairs) Sort()         { sort.Sort(kvs) }

//----------------------------------------
// KI64Pair

/*
Defined in types.proto
type KI64Pair struct {
	Key   []byte
	Value int64
}
*/

type KI64Pairs []KI64Pair

// Sorting
func (kvs KI64Pairs) Len() int { return len(kvs) }
func (kvs KI64Pairs) Less(i, j int) bool {
	switch bytes.Compare(kvs[i].Key, kvs[j].Key) {
	case -1:
		return true
	case 0:
		return kvs[i].Value < kvs[j].Value
	case 1:
		return false
	default:
		panic("invalid comparison result")
	}
}
func (kvs KI64Pairs) Swap(i, j int) { kvs[i], kvs[j] = kvs[j], kvs[i] }
func (kvs KI64Pairs) Sort()         { sort.Sort(kvs) }
