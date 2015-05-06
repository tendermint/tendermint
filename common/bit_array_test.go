package common

import (
	"testing"
)

func randBitArray(bits uint) (*BitArray, []byte) {
	src := RandBytes(int((bits + 7) / 8))
	bA := NewBitArray(bits)
	for i := uint(0); i < uint(len(src)); i++ {
		for j := uint(0); j < 8; j++ {
			if i*8+j >= bits {
				return bA, src
			}
			setBit := src[i]&(1<<j) > 0
			bA.SetIndex(i*8+j, setBit)
		}
	}
	return bA, src
}

func TestAnd(t *testing.T) {

	bA1, _ := randBitArray(51)
	bA2, _ := randBitArray(31)
	bA3 := bA1.And(bA2)

	if bA3.Bits != 31 {
		t.Error("Expected min bits", bA3.Bits)
	}
	if len(bA3.Elems) != len(bA2.Elems) {
		t.Error("Expected min elems length")
	}
	for i := uint(0); i < bA3.Bits; i++ {
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

	if bA3.Bits != 51 {
		t.Error("Expected max bits")
	}
	if len(bA3.Elems) != len(bA1.Elems) {
		t.Error("Expected max elems length")
	}
	for i := uint(0); i < bA3.Bits; i++ {
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

	if bA3.Bits != bA1.Bits {
		t.Error("Expected bA1 bits")
	}
	if len(bA3.Elems) != len(bA1.Elems) {
		t.Error("Expected bA1 elems length")
	}
	for i := uint(0); i < bA3.Bits; i++ {
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

	if bA3.Bits != bA1.Bits {
		t.Error("Expected bA1 bits")
	}
	if len(bA3.Elems) != len(bA1.Elems) {
		t.Error("Expected bA1 elems length")
	}
	for i := uint(0); i < bA3.Bits; i++ {
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
		bA1.SetIndex(uint(idx), true)
		index, ok := bA1.PickRandom()
		if !ok {
			t.Fatal("Expected to pick element but got none")
		}
		if index != uint(idx) {
			t.Fatalf("Expected to pick element at %v but got wrong index", idx)
		}
	}
}
