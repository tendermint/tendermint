package common

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func randBitArray(bits int) (*BitArray, []byte) {
	src := RandBytes((bits + 7) / 8)
	bA := NewBitArray(bits)
	for i := 0; i < len(src); i++ {
		for j := 0; j < 8; j++ {
			if i*8+j >= bits {
				return bA, src
			}
			setBit := src[i]&(1<<uint(j)) > 0
			bA.SetIndex(i*8+j, setBit)
		}
	}
	return bA, src
}

func TestAnd(t *testing.T) {

	bA1, _ := randBitArray(51)
	bA2, _ := randBitArray(31)
	bA3 := bA1.And(bA2)

	var bNil *BitArray
	require.Equal(t, bNil.And(bA1), (*BitArray)(nil))
	require.Equal(t, bA1.And(nil), (*BitArray)(nil))
	require.Equal(t, bNil.And(nil), (*BitArray)(nil))

	if bA3.Bits != 31 {
		t.Error("Expected min bits", bA3.Bits)
	}
	if len(bA3.Elems) != len(bA2.Elems) {
		t.Error("Expected min elems length")
	}
	for i := 0; i < bA3.Bits; i++ {
		expected := bA1.GetIndex(i) && bA2.GetIndex(i)
		if bA3.GetIndex(i) != expected {
			t.Error("Wrong bit from bA3", i, bA1.GetIndex(i), bA2.GetIndex(i), bA3.GetIndex(i))
		}
	}
}

func TestOr(t *testing.T) {

	bA1, _ := randBitArray(51)
	bA2, _ := randBitArray(31)
	bA3 := bA1.Or(bA2)

	bNil := (*BitArray)(nil)
	require.Equal(t, bNil.Or(bA1), bA1)
	require.Equal(t, bA1.Or(nil), bA1)
	require.Equal(t, bNil.Or(nil), (*BitArray)(nil))

	if bA3.Bits != 51 {
		t.Error("Expected max bits")
	}
	if len(bA3.Elems) != len(bA1.Elems) {
		t.Error("Expected max elems length")
	}
	for i := 0; i < bA3.Bits; i++ {
		expected := bA1.GetIndex(i) || bA2.GetIndex(i)
		if bA3.GetIndex(i) != expected {
			t.Error("Wrong bit from bA3", i, bA1.GetIndex(i), bA2.GetIndex(i), bA3.GetIndex(i))
		}
	}
}

func TestSub1(t *testing.T) {

	bA1, _ := randBitArray(31)
	bA2, _ := randBitArray(51)
	bA3 := bA1.Sub(bA2)

	bNil := (*BitArray)(nil)
	require.Equal(t, bNil.Sub(bA1), (*BitArray)(nil))
	require.Equal(t, bA1.Sub(nil), (*BitArray)(nil))
	require.Equal(t, bNil.Sub(nil), (*BitArray)(nil))

	if bA3.Bits != bA1.Bits {
		t.Error("Expected bA1 bits")
	}
	if len(bA3.Elems) != len(bA1.Elems) {
		t.Error("Expected bA1 elems length")
	}
	for i := 0; i < bA3.Bits; i++ {
		expected := bA1.GetIndex(i)
		if bA2.GetIndex(i) {
			expected = false
		}
		if bA3.GetIndex(i) != expected {
			t.Error("Wrong bit from bA3", i, bA1.GetIndex(i), bA2.GetIndex(i), bA3.GetIndex(i))
		}
	}
}

func TestSub2(t *testing.T) {

	bA1, _ := randBitArray(51)
	bA2, _ := randBitArray(31)
	bA3 := bA1.Sub(bA2)

	bNil := (*BitArray)(nil)
	require.Equal(t, bNil.Sub(bA1), (*BitArray)(nil))
	require.Equal(t, bA1.Sub(nil), (*BitArray)(nil))
	require.Equal(t, bNil.Sub(nil), (*BitArray)(nil))

	if bA3.Bits != bA1.Bits {
		t.Error("Expected bA1 bits")
	}
	if len(bA3.Elems) != len(bA1.Elems) {
		t.Error("Expected bA1 elems length")
	}
	for i := 0; i < bA3.Bits; i++ {
		expected := bA1.GetIndex(i)
		if i < bA2.Bits && bA2.GetIndex(i) {
			expected = false
		}
		if bA3.GetIndex(i) != expected {
			t.Error("Wrong bit from bA3")
		}
	}
}

func TestPickRandom(t *testing.T) {
	for idx := 0; idx < 123; idx++ {
		bA1 := NewBitArray(123)
		bA1.SetIndex(idx, true)
		index, ok := bA1.PickRandom()
		if !ok {
			t.Fatal("Expected to pick element but got none")
		}
		if index != idx {
			t.Fatalf("Expected to pick element at %v but got wrong index", idx)
		}
	}
}

func TestBytes(t *testing.T) {
	bA := NewBitArray(4)
	bA.SetIndex(0, true)
	check := func(bA *BitArray, bz []byte) {
		if !bytes.Equal(bA.Bytes(), bz) {
			panic(Fmt("Expected %X but got %X", bz, bA.Bytes()))
		}
	}
	check(bA, []byte{0x01})
	bA.SetIndex(3, true)
	check(bA, []byte{0x09})

	bA = NewBitArray(9)
	check(bA, []byte{0x00, 0x00})
	bA.SetIndex(7, true)
	check(bA, []byte{0x80, 0x00})
	bA.SetIndex(8, true)
	check(bA, []byte{0x80, 0x01})

	bA = NewBitArray(16)
	check(bA, []byte{0x00, 0x00})
	bA.SetIndex(7, true)
	check(bA, []byte{0x80, 0x00})
	bA.SetIndex(8, true)
	check(bA, []byte{0x80, 0x01})
	bA.SetIndex(9, true)
	check(bA, []byte{0x80, 0x03})
}

func TestEmptyFull(t *testing.T) {
	ns := []int{47, 123}
	for _, n := range ns {
		bA := NewBitArray(n)
		if !bA.IsEmpty() {
			t.Fatal("Expected bit array to be empty")
		}
		for i := 0; i < n; i++ {
			bA.SetIndex(i, true)
		}
		if !bA.IsFull() {
			t.Fatal("Expected bit array to be full")
		}
	}
}

func TestUpdateNeverPanics(t *testing.T) {
	newRandBitArray := func(n int) *BitArray {
		ba, _ := randBitArray(n)
		return ba
	}
	pairs := []struct {
		a, b *BitArray
	}{
		{nil, nil},
		{newRandBitArray(10), newRandBitArray(12)},
		{newRandBitArray(23), newRandBitArray(23)},
		{newRandBitArray(37), nil},
		{nil, NewBitArray(10)},
	}

	for _, pair := range pairs {
		a, b := pair.a, pair.b
		a.Update(b)
		b.Update(a)
	}
}

func TestNewBitArrayNeverCrashesOnNegatives(t *testing.T) {
	bitList := []int{-127, -128, -1 << 31}
	for _, bits := range bitList {
		_ = NewBitArray(bits)
	}
}

func TestJSONMarshalUnmarshal(t *testing.T) {

	bA1 := NewBitArray(0)

	bA2 := NewBitArray(1)

	bA3 := NewBitArray(1)
	bA3.SetIndex(0, true)

	bA4 := NewBitArray(5)
	bA4.SetIndex(0, true)
	bA4.SetIndex(1, true)

	testCases := []struct {
		bA           *BitArray
		marshalledBA string
	}{
		{nil, `null`},
		{bA1, `null`},
		{bA2, `"_"`},
		{bA3, `"x"`},
		{bA4, `"xx___"`},
	}

	for _, tc := range testCases {
		t.Run(tc.bA.String(), func(t *testing.T) {
			bz, err := json.Marshal(tc.bA)
			require.NoError(t, err)

			assert.Equal(t, tc.marshalledBA, string(bz))

			var unmarshalledBA *BitArray
			err = json.Unmarshal(bz, &unmarshalledBA)
			require.NoError(t, err)

			if tc.bA == nil {
				require.Nil(t, unmarshalledBA)
			} else {
				require.NotNil(t, unmarshalledBA)
				assert.EqualValues(t, tc.bA.Bits, unmarshalledBA.Bits)
				if assert.EqualValues(t, tc.bA.String(), unmarshalledBA.String()) {
					assert.EqualValues(t, tc.bA.Elems, unmarshalledBA.Elems)
				}
			}
		})
	}
}
