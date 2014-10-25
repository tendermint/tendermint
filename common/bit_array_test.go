package common

import (
	"bytes"
	"testing"
)

func randBitArray(bits uint) (BitArray, []byte) {
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

func TestReadWriteEmptyBitarray(t *testing.T) {
	bA1 := BitArray{}
	buf := new(bytes.Buffer)
	_, err := bA1.WriteTo(buf)
	if err != nil {
		t.Error("Failed to write empty bitarray")
	}

	var n int64
	bA2 := ReadBitArray(buf, &n, &err)
	if err != nil {
		t.Error("Failed to read empty bitarray")
	}
	if bA2.bits != 0 {
		t.Error("Expected to get bA2.bits 0")
	}
}

func TestReadWriteBitarray(t *testing.T) {

	// Make random bA1
	bA1, testData := randBitArray(64*10 + 8) // not divisible by 64

	// Write it
	buf := new(bytes.Buffer)
	_, err := bA1.WriteTo(buf)
	if err != nil {
		t.Error("Failed to write bitarray")
	}

	// Read it
	var n int64
	bA2 := ReadBitArray(buf, &n, &err)
	if err != nil {
		t.Error("Failed to read bitarray")
	}
	testData2 := make([]byte, len(testData))
	for i := uint(0); i < uint(len(testData)); i++ {
		for j := uint(0); j < 8; j++ {
			if bA2.GetIndex(i*8 + j) {
				testData2[i] |= 1 << j
			}
		}
	}

	// Compare testData
	if !bytes.Equal(testData, testData2) {
		t.Errorf("Not the same:\n%X\n%X", testData, testData2)
	}
}

func TestAnd(t *testing.T) {

	bA1, _ := randBitArray(51)
	bA2, _ := randBitArray(31)
	bA3 := bA1.And(bA2)

	if bA3.bits != 31 {
		t.Error("Expected min bits", bA3.bits)
	}
	if len(bA3.elems) != len(bA2.elems) {
		t.Error("Expected min elems length")
	}
	for i := uint(0); i < bA3.bits; i++ {
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

	if bA3.bits != 51 {
		t.Error("Expected max bits")
	}
	if len(bA3.elems) != len(bA1.elems) {
		t.Error("Expected max elems length")
	}
	for i := uint(0); i < bA3.bits; i++ {
		expected := bA1.GetIndex(i) || bA2.GetIndex(i)
		if bA3.GetIndex(i) != expected {
			t.Error("Wrong bit from bA3", i, bA1.GetIndex(i), bA2.GetIndex(i), bA3.GetIndex(i))
		}
	}
}

func TestSub(t *testing.T) {

	bA1, _ := randBitArray(31)
	bA2, _ := randBitArray(51)
	bA3 := bA1.Sub(bA2)

	if bA3.bits != 31 {
		t.Error("Expected min bits")
	}
	if len(bA3.elems) != len(bA1.elems) {
		t.Error("Expected min elems length")
	}
	for i := uint(0); i < bA3.bits; i++ {
		expected := bA1.GetIndex(i)
		if bA2.GetIndex(i) {
			expected = false
		}
		if bA3.GetIndex(i) != expected {
			t.Error("Wrong bit from bA3", i, bA1.GetIndex(i), bA2.GetIndex(i), bA3.GetIndex(i))
		}
	}
}
