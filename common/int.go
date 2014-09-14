package common

import (
	"sort"
)

// Sort for []uint64

type UInt64Slice []uint64

func (p UInt64Slice) Len() int           { return len(p) }
func (p UInt64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p UInt64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p UInt64Slice) Sort()              { sort.Sort(p) }

func SearchUInt64s(a []uint64, x uint64) int {
	return sort.Search(len(a), func(i int) bool { return a[i] >= x })
}

func (p UInt64Slice) Search(x uint64) int { return SearchUInt64s(p, x) }
