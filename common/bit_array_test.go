package common

import (
	"bytes"
	"fmt"
	"testing"

	. "github.com/tendermint/tendermint/binary"
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
	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	WriteBinary(bA1, buf, n, err)
	if *err != nil {
		t.Error("Failed to write empty bitarray")
	}

	bA2 := ReadBinary(BitArray{}, buf, n, err).(BitArray)
	if *err != nil {
		t.Error("Failed to read empty bitarray")
	}
	if bA2.Bits != 0 {
		t.Error("Expected to get bA2.Bits 0")
	}
}

func TestReadWriteBitarray(t *testing.T) {

	// Make random bA1
	bA1, testData := randBitArray(64*10 + 8) // not divisible by 64

	// Write it
	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	WriteBinary(bA1, buf, n, err)
	if *err != nil {
		t.Error("Failed to write bitarray")
	}

	fmt.Printf("Bytes: %X", buf.Bytes())

	// Read it
	bA2 := ReadBinary(BitArray{}, buf, n, err).(BitArray)
	if *err != nil {
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
