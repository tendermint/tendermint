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

//-----------------------------------------------------------------------------

func PutUint64(dest []byte, i uint64) {
	binary.LittleEndian.PutUint64(dest, i)
}

func GetUint64(src []byte) uint64 {
	return binary.LittleEndian.Uint64(src)
}
