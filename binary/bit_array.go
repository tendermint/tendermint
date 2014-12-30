package binary

import (
	"fmt"
	"math/rand"
	"strings"

	. "github.com/tendermint/tendermint/common"
)

// Not goroutine safe
type BitArray struct {
	Bits  uint     // NOTE: persisted via reflect, must be exported
	Elems []uint64 // NOTE: persisted via reflect, must be exported
}

func NewBitArray(bits uint) BitArray {
	return BitArray{bits, make([]uint64, (bits+63)/64)}
}

func (bA BitArray) Size() uint {
	return bA.Bits
}

func (bA BitArray) IsZero() bool {
	return bA.Bits == 0
}

// NOTE: behavior is undefined if i >= bA.Bits
func (bA BitArray) GetIndex(i uint) bool {
	if i >= bA.Bits {
		return false
	}
	return bA.Elems[i/64]&uint64(1<<(i%64)) > 0
}

// NOTE: behavior is undefined if i >= bA.Bits
func (bA BitArray) SetIndex(i uint, v bool) bool {
	if i >= bA.Bits {
		return false
	}
	if v {
		bA.Elems[i/64] |= uint64(1 << (i % 64))
	} else {
		bA.Elems[i/64] &= ^uint64(1 << (i % 64))
	}
	return true
}

func (bA BitArray) Copy() BitArray {
	c := make([]uint64, len(bA.Elems))
	copy(c, bA.Elems)
	return BitArray{bA.Bits, c}
}

func (bA BitArray) copyBits(bits uint) BitArray {
	c := make([]uint64, (bits+63)/64)
	copy(c, bA.Elems)
	return BitArray{bits, c}
}

// Returns a BitArray of larger bits size.
func (bA BitArray) Or(o BitArray) BitArray {
	c := bA.copyBits(MaxUint(bA.Bits, o.Bits))
	for i := 0; i < len(c.Elems); i++ {
		c.Elems[i] |= o.Elems[i]
	}
	return c
}

// Returns a BitArray of smaller bit size.
func (bA BitArray) And(o BitArray) BitArray {
	c := bA.copyBits(MinUint(bA.Bits, o.Bits))
	for i := 0; i < len(c.Elems); i++ {
		c.Elems[i] &= o.Elems[i]
	}
	return c
}

func (bA BitArray) Not() BitArray {
	c := bA.Copy()
	for i := 0; i < len(c.Elems); i++ {
		c.Elems[i] = ^c.Elems[i]
	}
	return c
}

func (bA BitArray) Sub(o BitArray) BitArray {
	if bA.Bits > o.Bits {
		c := bA.Copy()
		for i := 0; i < len(o.Elems)-1; i++ {
			c.Elems[i] &= ^c.Elems[i]
		}
		i := uint(len(o.Elems) - 1)
		if i >= 0 {
			for idx := i * 64; idx < o.Bits; idx++ {
				c.SetIndex(idx, c.GetIndex(idx) && !o.GetIndex(idx))
			}
		}
		return c
	} else {
		return bA.And(o.Not())
	}
}

func (bA BitArray) PickRandom() (uint, bool) {
	length := len(bA.Elems)
	if length == 0 {
		return 0, false
	}
	randElemStart := rand.Intn(length)
	for i := 0; i < length; i++ {
		elemIdx := ((i + randElemStart) % length)
		if elemIdx < length-1 {
			if bA.Elems[elemIdx] > 0 {
				randBitStart := rand.Intn(64)
				for j := 0; j < 64; j++ {
					bitIdx := ((j + randBitStart) % 64)
					if (bA.Elems[elemIdx] & (1 << uint(bitIdx))) > 0 {
						return 64*uint(elemIdx) + uint(bitIdx), true
					}
				}
				panic("should not happen")
			}
		} else {
			// Special case for last elem, to ignore straggler bits
			elemBits := int(bA.Bits) % 64
			if elemBits == 0 {
				elemBits = 64
			}
			randBitStart := rand.Intn(elemBits)
			for j := 0; j < elemBits; j++ {
				bitIdx := ((j + randBitStart) % elemBits)
				if (bA.Elems[elemIdx] & (1 << uint(bitIdx))) > 0 {
					return 64*uint(elemIdx) + uint(bitIdx), true
				}
			}
		}
	}
	return 0, false
}

func (bA BitArray) String() string {
	return bA.StringIndented("")
}

func (bA BitArray) StringIndented(indent string) string {
	lines := []string{}
	bits := ""
	for i := uint(0); i < bA.Bits; i++ {
		if bA.GetIndex(i) {
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
	return fmt.Sprintf("BA{%v:%v}", bA.Bits, strings.Join(lines, indent))
}
