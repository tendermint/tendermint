package common

import (
	"io"
	"math"
	"math/rand"
	"strings"

	. "github.com/tendermint/tendermint/binary"
)

// Not goroutine safe
type BitArray []uint64

func NewBitArray(length uint) BitArray {
	return BitArray(make([]uint64, (length+63)/64))
}

func ReadBitArray(r io.Reader, n *int64, err *error) BitArray {
	lengthTotal := ReadUInt32(r, n, err)
	lengthWritten := ReadUInt32(r, n, err)
	if *err != nil {
		return nil
	}
	buf := make([]uint64, int(lengthTotal))
	for i := uint32(0); i < lengthWritten; i++ {
		buf[i] = ReadUInt64(r, n, err)
		if err != nil {
			return nil
		}
	}
	return BitArray(buf)
}

func (bA BitArray) WriteTo(w io.Writer) (n int64, err error) {
	// Count the last element > 0.
	lastNonzeroIndex := -1
	for i, elem := range bA {
		if elem > 0 {
			lastNonzeroIndex = i
		}
	}
	WriteUInt32(w, uint32(len(bA)), &n, &err)
	WriteUInt32(w, uint32(lastNonzeroIndex+1), &n, &err)
	for i, elem := range bA {
		if i > lastNonzeroIndex {
			break
		}
		WriteUInt64(w, elem, &n, &err)
	}
	return
}

func (bA BitArray) GetIndex(i uint) bool {
	return bA[i/64]&uint64(1<<(i%64)) > 0
}

func (bA BitArray) SetIndex(i uint, v bool) {
	if v {
		bA[i/64] |= uint64(1 << (i % 64))
	} else {
		bA[i/64] &= ^uint64(1 << (i % 64))
	}
}

func (bA BitArray) Copy() BitArray {
	c := make([]uint64, len(bA))
	copy(c, bA)
	return BitArray(c)
}

func (bA BitArray) Or(o BitArray) BitArray {
	c := bA.Copy()
	for i, _ := range c {
		c[i] = o[i] | c[i]
	}
	return c
}

func (bA BitArray) And(o BitArray) BitArray {
	c := bA.Copy()
	for i, _ := range c {
		c[i] = o[i] & c[i]
	}
	return c
}

func (bA BitArray) Not() BitArray {
	c := bA.Copy()
	for i, _ := range c {
		c[i] = ^c[i]
	}
	return c
}

func (bA BitArray) Sub(o BitArray) BitArray {
	return bA.And(o.Not())
}

// NOTE: returns counts or a longer int slice as necessary.
func (bA BitArray) AddToCounts(counts []int) []int {
	for bytei := 0; bytei < len(bA); bytei++ {
		for biti := 0; biti < 64; biti++ {
			if (bA[bytei] & (1 << uint(biti))) == 0 {
				continue
			}
			index := 64*bytei + biti
			if len(counts) <= index {
				counts = append(counts, make([]int, (index-len(counts)+1))...)
			}
			counts[index]++
		}
	}
	return counts
}

func (bA BitArray) PickRandom() (int, bool) {
	randStart := rand.Intn(len(bA))
	for i := 0; i < len(bA); i++ {
		bytei := ((i + randStart) % len(bA))
		if bA[bytei] > 0 {
			randBitStart := rand.Intn(64)
			for j := 0; j < 64; j++ {
				biti := ((j + randBitStart) % 64)
				//fmt.Printf("%X %v %v %v\n", iHas, j, biti, randBitStart)
				if (bA[bytei] & (1 << uint(biti))) > 0 {
					return 64*int(bytei) + int(biti), true
				}
			}
			panic("should not happen")
		}
	}
	return 0, false
}

// Pick an index from this BitArray that is 1 && whose count is lowest.
func (bA BitArray) PickRarest(counts []int) (rarest int, ok bool) {
	smallestCount := math.MaxInt32
	for bytei := 0; bytei < len(bA); bytei++ {
		if bA[bytei] > 0 {
			for biti := 0; biti < 64; biti++ {
				if (bA[bytei] & (1 << uint(biti))) == 0 {
					continue
				}
				index := 64*bytei + biti
				if counts[index] < smallestCount {
					smallestCount = counts[index]
					rarest = index
					ok = true
				}
			}
			panic("should not happen")
		}
	}
	return
}

func (bA BitArray) String() string {
	return bA.StringWithIndent("")
}

func (bA BitArray) StringWithIndent(indent string) string {
	lines := []string{}
	bits := ""
	for i := 0; i < len(bA)*64; i++ {
		if bA.GetIndex(uint(i)) {
			bits += "X"
		} else {
			bits += "_"
		}
		if i%100 == 99 {
			lines = append(lines, bits)
			bits = ""
		}
		if i%10 == 9 {
			bits += " "
		}
		if i%50 == 49 {
			bits += " "
		}
	}
	if len(bits) > 0 {
		lines = append(lines, bits)
	}
	return strings.Join(lines, indent)
}
