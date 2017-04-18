package common

import (
	"encoding/binary"
	"sort"
)

// Sort for []uint64

type Uint64Slice []uint64

func (p Uint64Slice) Len() int           { return len(p) }
func (p Uint64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p Uint64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p Uint64Slice) Sort()              { sort.Sort(p) }

func SearchUint64s(a []uint64, x uint64) int {
	return sort.Search(len(a), func(i int) bool { return a[i] >= x })
}

func (p Uint64Slice) Search(x uint64) int { return SearchUint64s(p, x) }

//--------------------------------------------------------------------------------

func PutUint64LE(dest []byte, i uint64) {
	binary.LittleEndian.PutUint64(dest, i)
}

func GetUint64LE(src []byte) uint64 {
	return binary.LittleEndian.Uint64(src)
}

func PutUint64BE(dest []byte, i uint64) {
	binary.BigEndian.PutUint64(dest, i)
}

func GetUint64BE(src []byte) uint64 {
	return binary.BigEndian.Uint64(src)
}

func PutInt64LE(dest []byte, i int64) {
	binary.LittleEndian.PutUint64(dest, uint64(i))
}

func GetInt64LE(src []byte) int64 {
	return int64(binary.LittleEndian.Uint64(src))
}

func PutInt64BE(dest []byte, i int64) {
	binary.BigEndian.PutUint64(dest, uint64(i))
}

func GetInt64BE(src []byte) int64 {
	return int64(binary.BigEndian.Uint64(src))
}
