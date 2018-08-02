package common

import (
	"encoding/binary"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// BitArray is a thread-safe implementation of a bit array.
type BitArray struct {
	mtx   sync.Mutex
	Bits  int      `json:"bits"`  // NOTE: persisted via reflect, must be exported
	Elems []uint64 `json:"elems"` // NOTE: persisted via reflect, must be exported
}

// NewBitArray returns a new bit array.
// It returns nil if the number of bits is zero.
func NewBitArray(bits int) *BitArray {
	if bits <= 0 {
		return nil
	}
	return &BitArray{
		Bits:  bits,
		Elems: make([]uint64, (bits+63)/64),
	}
}

// Size returns the number of bits in the bitarray
func (bA *BitArray) Size() int {
	if bA == nil {
		return 0
	}
	return bA.Bits
}

// GetIndex returns the bit at index i within the bit array.
// The behavior is undefined if i >= bA.Bits
func (bA *BitArray) GetIndex(i int) bool {
	if bA == nil {
		return false
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.getIndex(i)
}

func (bA *BitArray) getIndex(i int) bool {
	if i >= bA.Bits {
		return false
	}
	return bA.Elems[i/64]&(uint64(1)<<uint(i%64)) > 0
}

// SetIndex sets the bit at index i within the bit array.
// The behavior is undefined if i >= bA.Bits
func (bA *BitArray) SetIndex(i int, v bool) bool {
	if bA == nil {
		return false
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.setIndex(i, v)
}

func (bA *BitArray) setIndex(i int, v bool) bool {
	if i >= bA.Bits {
		return false
	}
	if v {
		bA.Elems[i/64] |= (uint64(1) << uint(i%64))
	} else {
		bA.Elems[i/64] &= ^(uint64(1) << uint(i%64))
	}
	return true
}

// Copy returns a copy of the provided bit array.
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

func (bA *BitArray) copyBits(bits int) *BitArray {
	c := make([]uint64, (bits+63)/64)
	copy(c, bA.Elems)
	return &BitArray{
		Bits:  bits,
		Elems: c,
	}
}

// Or returns a bit array resulting from a bitwise OR of the two bit arrays.
// If the two bit-arrys have different lengths, Or right-pads the smaller of the two bit-arrays with zeroes.
// Thus the size of the return value is the maximum of the two provided bit arrays.
func (bA *BitArray) Or(o *BitArray) *BitArray {
	if bA == nil && o == nil {
		return nil
	}
	if bA == nil && o != nil {
		return o.Copy()
	}
	if o == nil {
		return bA.Copy()
	}
	bA.mtx.Lock()
	o.mtx.Lock()
	defer func() {
		bA.mtx.Unlock()
		o.mtx.Unlock()
	}()
	c := bA.copyBits(MaxInt(bA.Bits, o.Bits))
	for i := 0; i < len(c.Elems); i++ {
		c.Elems[i] |= o.Elems[i]
	}
	return c
}

// And returns a bit array resulting from a bitwise AND of the two bit arrays.
// If the two bit-arrys have different lengths, this truncates the larger of the two bit-arrays from the right.
// Thus the size of the return value is the minimum of the two provided bit arrays.
func (bA *BitArray) And(o *BitArray) *BitArray {
	if bA == nil || o == nil {
		return nil
	}
	bA.mtx.Lock()
	o.mtx.Lock()
	defer func() {
		bA.mtx.Unlock()
		o.mtx.Unlock()
	}()
	return bA.and(o)
}

func (bA *BitArray) and(o *BitArray) *BitArray {
	c := bA.copyBits(MinInt(bA.Bits, o.Bits))
	for i := 0; i < len(c.Elems); i++ {
		c.Elems[i] &= o.Elems[i]
	}
	return c
}

// Not returns a bit array resulting from a bitwise Not of the provided bit array.
func (bA *BitArray) Not() *BitArray {
	if bA == nil {
		return nil // Degenerate
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.not()
}

func (bA *BitArray) not() *BitArray {
	c := bA.copy()
	for i := 0; i < len(c.Elems); i++ {
		c.Elems[i] = ^c.Elems[i]
	}
	return c
}

// Sub subtracts the two bit-arrays bitwise, without carrying the bits.
// This is essentially bA.And(o.Not()).
// If bA is longer than o, o is right padded with zeroes.
func (bA *BitArray) Sub(o *BitArray) *BitArray {
	if bA == nil || o == nil {
		// TODO: Decide if we should do 1's complement here?
		return nil
	}
	bA.mtx.Lock()
	o.mtx.Lock()
	defer func() {
		bA.mtx.Unlock()
		o.mtx.Unlock()
	}()
	if bA.Bits > o.Bits {
		c := bA.copy()
		for i := 0; i < len(o.Elems)-1; i++ {
			c.Elems[i] &= ^c.Elems[i]
		}
		i := len(o.Elems) - 1
		if i >= 0 {
			for idx := i * 64; idx < o.Bits; idx++ {
				c.setIndex(idx, c.getIndex(idx) && !o.getIndex(idx))
			}
		}
		return c
	}
	return bA.and(o.not()) // Note degenerate case where o == nil
}

// IsEmpty returns true iff all bits in the bit array are 0
func (bA *BitArray) IsEmpty() bool {
	if bA == nil {
		return true // should this be opposite?
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	for _, e := range bA.Elems {
		if e > 0 {
			return false
		}
	}
	return true
}

// IsFull returns true iff all bits in the bit array are 1.
func (bA *BitArray) IsFull() bool {
	if bA == nil {
		return true
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()

	// Check all elements except the last
	for _, elem := range bA.Elems[:len(bA.Elems)-1] {
		if (^elem) != 0 {
			return false
		}
	}

	// Check that the last element has (lastElemBits) 1's
	lastElemBits := (bA.Bits+63)%64 + 1
	lastElem := bA.Elems[len(bA.Elems)-1]
	return (lastElem+1)&((uint64(1)<<uint(lastElemBits))-1) == 0
}

// PickRandom returns a random index in the bit array, and its value.
// It uses the global randomness in `random.go` to get this index.
func (bA *BitArray) PickRandom() (int, bool) {
	if bA == nil {
		return 0, false
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()

	length := len(bA.Elems)
	if length == 0 {
		return 0, false
	}
	randElemStart := RandIntn(length)
	for i := 0; i < length; i++ {
		elemIdx := ((i + randElemStart) % length)
		if elemIdx < length-1 {
			if bA.Elems[elemIdx] > 0 {
				randBitStart := RandIntn(64)
				for j := 0; j < 64; j++ {
					bitIdx := ((j + randBitStart) % 64)
					if (bA.Elems[elemIdx] & (uint64(1) << uint(bitIdx))) > 0 {
						return 64*elemIdx + bitIdx, true
					}
				}
				PanicSanity("should not happen")
			}
		} else {
			// Special case for last elem, to ignore straggler bits
			elemBits := bA.Bits % 64
			if elemBits == 0 {
				elemBits = 64
			}
			randBitStart := RandIntn(elemBits)
			for j := 0; j < elemBits; j++ {
				bitIdx := ((j + randBitStart) % elemBits)
				if (bA.Elems[elemIdx] & (uint64(1) << uint(bitIdx))) > 0 {
					return 64*elemIdx + bitIdx, true
				}
			}
		}
	}
	return 0, false
}

// String returns a string representation of BitArray: BA{<bit-string>},
// where <bit-string> is a sequence of 'x' (1) and '_' (0).
// The <bit-string> includes spaces and newlines to help people.
// For a simple sequence of 'x' and '_' characters with no spaces or newlines,
// see the MarshalJSON() method.
// Example: "BA{_x_}" or "nil-BitArray" for nil.
func (bA *BitArray) String() string {
	return bA.StringIndented("")
}

// StringIndented returns the same thing as String(), but applies the indent
// at every 10th bit, and twice at every 50th bit.
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
	for i := 0; i < bA.Bits; i++ {
		if bA.getIndex(i) {
			bits += "x"
		} else {
			bits += "_"
		}
		if i%100 == 99 {
			lines = append(lines, bits)
			bits = ""
		}
		if i%10 == 9 {
			bits += indent
		}
		if i%50 == 49 {
			bits += indent
		}
	}
	if len(bits) > 0 {
		lines = append(lines, bits)
	}
	return fmt.Sprintf("BA{%v:%v}", bA.Bits, strings.Join(lines, indent))
}

// Bytes returns the byte representation of the bits within the bitarray.
func (bA *BitArray) Bytes() []byte {
	bA.mtx.Lock()
	defer bA.mtx.Unlock()

	numBytes := (bA.Bits + 7) / 8
	bytes := make([]byte, numBytes)
	for i := 0; i < len(bA.Elems); i++ {
		elemBytes := [8]byte{}
		binary.LittleEndian.PutUint64(elemBytes[:], bA.Elems[i])
		copy(bytes[i*8:], elemBytes[:])
	}
	return bytes
}

// Update sets the bA's bits to be that of the other bit array.
// The copying begins from the begin of both bit arrays.
func (bA *BitArray) Update(o *BitArray) {
	if bA == nil || o == nil {
		return
	}
	bA.mtx.Lock()
	o.mtx.Lock()
	defer func() {
		bA.mtx.Unlock()
		o.mtx.Unlock()
	}()

	copy(bA.Elems, o.Elems)
}

// MarshalJSON implements json.Marshaler interface by marshaling bit array
// using a custom format: a string of '-' or 'x' where 'x' denotes the 1 bit.
func (bA *BitArray) MarshalJSON() ([]byte, error) {
	if bA == nil {
		return []byte("null"), nil
	}

	bA.mtx.Lock()
	defer bA.mtx.Unlock()

	bits := `"`
	for i := 0; i < bA.Bits; i++ {
		if bA.getIndex(i) {
			bits += `x`
		} else {
			bits += `_`
		}
	}
	bits += `"`
	return []byte(bits), nil
}

var bitArrayJSONRegexp = regexp.MustCompile(`\A"([_x]*)"\z`)

// UnmarshalJSON implements json.Unmarshaler interface by unmarshaling a custom
// JSON description.
func (bA *BitArray) UnmarshalJSON(bz []byte) error {
	b := string(bz)
	if b == "null" {
		// This is required e.g. for encoding/json when decoding
		// into a pointer with pre-allocated BitArray.
		bA.Bits = 0
		bA.Elems = nil
		return nil
	}

	// Validate 'b'.
	match := bitArrayJSONRegexp.FindStringSubmatch(b)
	if match == nil {
		return fmt.Errorf("BitArray in JSON should be a string of format %q but got %s", bitArrayJSONRegexp.String(), b)
	}
	bits := match[1]

	// Construct new BitArray and copy over.
	numBits := len(bits)
	bA2 := NewBitArray(numBits)
	for i := 0; i < numBits; i++ {
		if bits[i] == 'x' {
			bA2.SetIndex(i, true)
		}
	}
	*bA = *bA2
	return nil
}
