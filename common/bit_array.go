package common

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
)

type BitArray struct {
	mtx   sync.Mutex
	Bits  uint     `json:"bits"`  // NOTE: persisted via reflect, must be exported
	Elems []uint64 `json:"elems"` // NOTE: persisted via reflect, must be exported
}

func NewBitArray(bits uint) *BitArray {
	return &BitArray{
		Bits:  bits,
		Elems: make([]uint64, (bits+63)/64),
	}
}

func (bA *BitArray) Size() uint {
	if bA == nil {
		return 0
	}
	return bA.Bits
}

// NOTE: behavior is undefined if i >= bA.Bits
func (bA *BitArray) GetIndex(i uint) bool {
	if bA == nil {
		return false
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.getIndex(i)
}

func (bA *BitArray) getIndex(i uint) bool {
	if i >= bA.Bits {
		return false
	}
	return bA.Elems[i/64]&(uint64(1)<<(i%64)) > 0
}

// NOTE: behavior is undefined if i >= bA.Bits
func (bA *BitArray) SetIndex(i uint, v bool) bool {
	if bA == nil {
		return false
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.setIndex(i, v)
}

func (bA *BitArray) setIndex(i uint, v bool) bool {
	if i >= bA.Bits {
		return false
	}
	if v {
		bA.Elems[i/64] |= (uint64(1) << (i % 64))
	} else {
		bA.Elems[i/64] &= ^(uint64(1) << (i % 64))
	}
	return true
}

func (bA *BitArray) Copy() *BitArray {
	if bA == nil {
		return nil
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.copy()
}

func (bA *BitArray) copy() *BitArray {
	c := make([]uint64, len(bA.Elems))
	copy(c, bA.Elems)
	return &BitArray{
		Bits:  bA.Bits,
		Elems: c,
	}
}

func (bA *BitArray) copyBits(bits uint) *BitArray {
	c := make([]uint64, (bits+63)/64)
	copy(c, bA.Elems)
	return &BitArray{
		Bits:  bits,
		Elems: c,
	}
}

// Returns a BitArray of larger bits size.
func (bA *BitArray) Or(o *BitArray) *BitArray {
	if bA == nil {
		o.Copy()
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	c := bA.copyBits(MaxUint(bA.Bits, o.Bits))
	for i := 0; i < len(c.Elems); i++ {
		c.Elems[i] |= o.Elems[i]
	}
	return c
}

// Returns a BitArray of smaller bit size.
func (bA *BitArray) And(o *BitArray) *BitArray {
	if bA == nil {
		return nil
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.and(o)
}

func (bA *BitArray) and(o *BitArray) *BitArray {
	c := bA.copyBits(MinUint(bA.Bits, o.Bits))
	for i := 0; i < len(c.Elems); i++ {
		c.Elems[i] &= o.Elems[i]
	}
	return c
}

func (bA *BitArray) Not() *BitArray {
	if bA == nil {
		return nil // Degenerate
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	c := bA.copy()
	for i := 0; i < len(c.Elems); i++ {
		c.Elems[i] = ^c.Elems[i]
	}
	return c
}

func (bA *BitArray) Sub(o *BitArray) *BitArray {
	if bA == nil {
		return nil
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	if bA.Bits > o.Bits {
		c := bA.copy()
		for i := 0; i < len(o.Elems)-1; i++ {
			c.Elems[i] &= ^c.Elems[i]
		}
		i := uint(len(o.Elems) - 1)
		if i >= 0 {
			for idx := i * 64; idx < o.Bits; idx++ {
				c.setIndex(idx, c.getIndex(idx) && !o.GetIndex(idx))
			}
		}
		return c
	} else {
		return bA.and(o.Not()) // Note degenerate case where o == nil
	}
}

func (bA *BitArray) IsFull() bool {
	if bA == nil {
		return true
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()

	if bA.Bits == 0 {
		return false
	}

	// Check all elements except the last
	for _, elem := range bA.Elems[:len(bA.Elems)-1] {
		if (^elem) != 0 {
			return false
		}
	}

	// Check that the last element has (lastElemBits) 1's
	lastElemBits := (bA.Bits+63)%64 + 1
	lastElem := bA.Elems[len(bA.Elems)-1]
	return (lastElem+1)&((uint64(1)<<lastElemBits)-1) == 0
}

func (bA *BitArray) PickRandom() (uint, bool) {
	if bA == nil {
		return 0, false
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()

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
					if (bA.Elems[elemIdx] & (uint64(1) << uint(bitIdx))) > 0 {
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
				if (bA.Elems[elemIdx] & (uint64(1) << uint(bitIdx))) > 0 {
					return 64*uint(elemIdx) + uint(bitIdx), true
				}
			}
		}
	}
	return 0, false
}

func (bA *BitArray) String() string {
	if bA == nil {
		return "nil-BitArray"
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.stringIndented("")
}

func (bA *BitArray) StringIndented(indent string) string {
	if bA == nil {
		return "nil-BitArray"
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.stringIndented(indent)
}

func (bA *BitArray) stringIndented(indent string) string {

	lines := []string{}
	bits := ""
	for i := uint(0); i < bA.Bits; i++ {
		if bA.getIndex(i) {
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
