package common

import (
	"fmt"
	"io"
	"math/rand"
	"strings"

	. "github.com/tendermint/tendermint/binary"
)

// Not goroutine safe
type BitArray struct {
	bits  uint
	elems []uint64
}

func NewBitArray(bits uint) BitArray {
	return BitArray{bits, make([]uint64, (bits+63)/64)}
}

func ReadBitArray(r io.Reader, n *int64, err *error) BitArray {
	bits := ReadUVarInt(r, n, err)
	if bits == 0 {
		return BitArray{}
	}
	elemsWritten := ReadUVarInt(r, n, err)
	if *err != nil {
		return BitArray{}
	}
	bA := NewBitArray(bits)
	for i := uint(0); i < elemsWritten; i++ {
		bA.elems[i] = ReadUInt64(r, n, err)
		if *err != nil {
			return BitArray{}
		}
	}
	return bA
}

func (bA BitArray) WriteTo(w io.Writer) (n int64, err error) {
	WriteUVarInt(w, bA.bits, &n, &err)
	if bA.bits == 0 {
		return
	}
	// Count the last element > 0.
	elemsToWrite := 0
	for i, elem := range bA.elems {
		if elem > 0 {
			elemsToWrite = i + 1
		}
	}
	WriteUVarInt(w, uint(elemsToWrite), &n, &err)
	for i, elem := range bA.elems {
		if i >= elemsToWrite {
			break
		}
		WriteUInt64(w, elem, &n, &err)
	}
	return
}

func (bA BitArray) Size() uint {
	return bA.bits
}

func (bA BitArray) IsZero() bool {
	return bA.bits == 0
}

// NOTE: behavior is undefined if i >= bA.bits
func (bA BitArray) GetIndex(i uint) bool {
	if i >= bA.bits {
		return false
	}
	return bA.elems[i/64]&uint64(1<<(i%64)) > 0
}

// NOTE: behavior is undefined if i >= bA.bits
func (bA BitArray) SetIndex(i uint, v bool) bool {
	if i >= bA.bits {
		return false
	}
	if v {
		bA.elems[i/64] |= uint64(1 << (i % 64))
	} else {
		bA.elems[i/64] &= ^uint64(1 << (i % 64))
	}
	return true
}

func (bA BitArray) Copy() BitArray {
	c := make([]uint64, len(bA.elems))
	copy(c, bA.elems)
	return BitArray{bA.bits, c}
}

func (bA BitArray) copyBits(bits uint) BitArray {
	c := make([]uint64, (bits+63)/64)
	copy(c, bA.elems)
	return BitArray{bits, c}
}

// Returns a BitArray of larger bits size.
func (bA BitArray) Or(o BitArray) BitArray {
	c := bA.copyBits(MaxUint(bA.bits, o.bits))
	for i := 0; i < len(c.elems); i++ {
		c.elems[i] |= o.elems[i]
	}
	return c
}

// Returns a BitArray of smaller bit size.
func (bA BitArray) And(o BitArray) BitArray {
	c := bA.copyBits(MinUint(bA.bits, o.bits))
	for i := 0; i < len(c.elems); i++ {
		c.elems[i] &= o.elems[i]
	}
	return c
}

func (bA BitArray) Not() BitArray {
	c := bA.Copy()
	for i := 0; i < len(c.elems); i++ {
		c.elems[i] = ^c.elems[i]
	}
	return c
}

func (bA BitArray) Sub(o BitArray) BitArray {
	if bA.bits > o.bits {
		c := bA.Copy()
		for i := 0; i < len(o.elems)-1; i++ {
			c.elems[i] &= ^c.elems[i]
		}
		i := uint(len(o.elems) - 1)
		if i >= 0 {
			for idx := i * 64; idx < o.bits; idx++ {
				c.SetIndex(idx, c.GetIndex(idx) && !o.GetIndex(idx))
			}
		}
		return c
	} else {
		return bA.And(o.Not())
	}
}

func (bA BitArray) PickRandom() (uint, bool) {
	length := len(bA.elems)
	if length == 0 {
		return 0, false
	}
	randElemStart := rand.Intn(length)
	for i := 0; i < length; i++ {
		elemIdx := ((i + randElemStart) % length)
		if elemIdx < length-1 {
			if bA.elems[elemIdx] > 0 {
				randBitStart := rand.Intn(64)
				for j := 0; j < 64; j++ {
					bitIdx := ((j + randBitStart) % 64)
					if (bA.elems[elemIdx] & (1 << uint(bitIdx))) > 0 {
						return 64*uint(elemIdx) + uint(bitIdx), true
					}
				}
				panic("should not happen")
			}
		} else {
			// Special case for last elem, to ignore straggler bits
			elemBits := int(bA.bits) % 64
			if elemBits == 0 {
				elemBits = 64
			}
			randBitStart := rand.Intn(elemBits)
			for j := 0; j < elemBits; j++ {
				bitIdx := ((j + randBitStart) % elemBits)
				if (bA.elems[elemIdx] & (1 << uint(bitIdx))) > 0 {
					return 64*uint(elemIdx) + uint(bitIdx), true
				}
			}
		}
	}
	return 0, false
}

func (bA BitArray) String() string {
	return bA.StringWithIndent("")
}

func (bA BitArray) StringWithIndent(indent string) string {
	lines := []string{}
	bits := ""
	for i := uint(0); i < bA.bits; i++ {
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
	return fmt.Sprintf("BA{%v:%v}", bA.bits, strings.Join(lines, indent))
}
