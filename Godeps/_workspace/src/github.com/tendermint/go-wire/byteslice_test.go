package wire

import (
	"bytes"
	"testing"
)

func TestReadByteSliceEquality(t *testing.T) {

	var buf = bytes.NewBuffer(nil)
	var bufBytes []byte

	// Write a byteslice
	var testBytes = []byte("ThisIsSomeTestArray")
	var n int
	var err error
	WriteByteSlice(testBytes, buf, &n, &err)
	if err != nil {
		t.Error(err.Error())
	}
	bufBytes = buf.Bytes()

	// Read the byteslice, should return the same byteslice
	buf = bytes.NewBuffer(bufBytes)
	var n2 int
	res := ReadByteSlice(buf, 0, &n2, &err)
	if err != nil {
		t.Error(err.Error())
	}
	if n != n2 {
		t.Error("Read bytes did not match write bytes length")
	}

	if !bytes.Equal(testBytes, res) {
		t.Error("Returned the wrong bytes")
	}

}

func TestReadByteSliceLimit(t *testing.T) {

	var buf = bytes.NewBuffer(nil)
	var bufBytes []byte

	// Write a byteslice
	var testBytes = []byte("ThisIsSomeTestArray")
	var n int
	var err error
	WriteByteSlice(testBytes, buf, &n, &err)
	if err != nil {
		t.Error(err.Error())
	}
	bufBytes = buf.Bytes()

	// Read the byteslice, should work fine with no limit.
	buf = bytes.NewBuffer(bufBytes)
	var n2 int
	ReadByteSlice(buf, 0, &n2, &err)
	if err != nil {
		t.Error(err.Error())
	}
	if n != n2 {
		t.Error("Read bytes did not match write bytes length")
	}

	// Limit to the byteslice length, should succeed.
	buf = bytes.NewBuffer(bufBytes)
	t.Logf("%X", bufBytes)
	var n3 int
	ReadByteSlice(buf, len(bufBytes), &n3, &err)
	if err != nil {
		t.Error(err.Error())
	}
	if n != n3 {
		t.Error("Read bytes did not match write bytes length")
	}

	// Limit to the byteslice length, should succeed.
	buf = bytes.NewBuffer(bufBytes)
	var n4 int
	ReadByteSlice(buf, len(bufBytes)-1, &n4, &err)
	if err != ErrBinaryReadOverflow {
		t.Error("Expected ErrBinaryReadsizeOverflow")
	}

}

func TestPutByteSlice(t *testing.T) {
	var buf []byte = make([]byte, 1000)
	var testBytes = []byte("ThisIsSomeTestArray")
	n, err := PutByteSlice(buf, testBytes)
	if err != nil {
		t.Error(err.Error())
	}
	if !bytes.Equal(buf[0:2], []byte{1, 19}) {
		t.Error("Expected first two bytes to encode varint 19")
	}
	if n != 21 {
		t.Error("Expected to write 21 bytes")
	}
	if !bytes.Equal(buf[2:21], testBytes) {
		t.Error("Expected last 19 bytes to encode string")
	}
	if !bytes.Equal(buf[21:], make([]byte, 1000-21)) {
		t.Error("Expected remaining bytes to be zero")
	}
}

func TestGetByteSlice(t *testing.T) {
	var buf []byte = make([]byte, 1000)
	var testBytes = []byte("ThisIsSomeTestArray")
	n, err := PutByteSlice(buf, testBytes)
	if err != nil {
		t.Error(err.Error())
	}

	got, n, err := GetByteSlice(buf)
	if err != nil {
		t.Error(err.Error())
	}
	if n != 21 {
		t.Error("Expected to read 21 bytes")
	}
	if !bytes.Equal(got, testBytes) {
		t.Error("Expected to read %v, got %v", testBytes, got)
	}
}
