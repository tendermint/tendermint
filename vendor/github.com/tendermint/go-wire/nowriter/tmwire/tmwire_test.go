package tmwire

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	cmn "github.com/tendermint/tmlibs/common"

	"github.com/tendermint/go-wire/nowriter/tmdecoding"
	"github.com/tendermint/go-wire/nowriter/tmencoding"
	"github.com/tendermint/go-wire/nowriter/tmlegacy"
)

var legacy = tmlegacy.TMEncoderLegacy{}
var pure = tmencoding.TMEncoderPure{}
var dec = tmdecoding.TMDecoderPure{}

func TestByte(t *testing.T) {
	for i := 0; i < 256; i += 1 {
		x0 := byte(i)
		buf1 := new(bytes.Buffer)
		n1, err1 := new(int), new(error)
		legacy.WriteOctet(x0, buf1, n1, err1)
		b1 := buf1.Bytes()
		b2 := pure.EncodeOctet(x0)
		if !bytes.Equal(b1, b2) {
			t.Fatalf("Bytes do not match for %#v and %#v", b1, b2)
		}
		b3, n3, err3 := dec.DecodeOctet(b1)
		if b3 != x0 {
			t.Fatalf("Decoded bytes do not match for %#v and %#v", b3, x0)
		}
		if n3 != *n1 {
			t.Fatalf("Decoded byte count is not correct")
		}
		assert.Nil(t, err3)
	}
}

func deactivatedTestBytes(t *testing.T) {
	cases := []int{1, 40, 255, 256, 257, 65530, 65535, 65536, 65537}

	for _, l0 := range cases {
		x0 := cmn.RandBytes(l0)
		buf1 := new(bytes.Buffer)
		n1, err1 := new(int), new(error)
		legacy.WriteOctetSlice(x0, buf1, n1, err1)
		b1 := buf1.Bytes()
		b2 := pure.EncodeOctets(x0)
		if !bytes.Equal(b1, b2) {
			t.Fatalf("Bytes do not match for %#v and %#v", b1, b2)
		}
		b3, n3, err3 := dec.DecodeOctets(b1)
		if !bytes.Equal(b3, x0) {
			t.Fatalf("Decoded bytes do not match for %#v and %#v", b3, x0)
		}
		if n3 != *n1 {
			t.Fatalf("Decoded byte count is not correct")
		}
		assert.Nil(t, err3)
	}
}

func TestUint16(t *testing.T) {
	for i := 0; i < 0x10000; i += 1 {
		x0 := uint16(i)
		buf1 := new(bytes.Buffer)
		n1, err1 := new(int), new(error)
		legacy.WriteUint16(x0, buf1, n1, err1)
		b1 := buf1.Bytes()
		b2 := pure.EncodeUint16(x0)
		if !bytes.Equal(b1, b2) {
			t.Fatalf("Bytes do not match for %#v and %#v", b1, b2)
		}
		b3, n3, err3 := dec.DecodeUint16(b1)
		if b3 != x0 {
			t.Fatalf("Decoded Uint16 do not match for %#v and %#v", b3, x0)
		}
		if n3 != *n1 {
			t.Fatalf("Decoded byte count is not correct: %d and %d", n3, *n1)
		}
		assert.Nil(t, err3)
	}
}

func deactivatedTestUint16s(t *testing.T) {
	// This should panic on 32-bit machine
	/*
		countBytes := []byte{0xff, 0xff, 0xff, 0xff}
		count := binary.BigEndian.Uint32(countBytes)
		fmt.Println(count, int32(count))
		size := int(2 * count)
		x0 := make([]byte, size+4)
		copy(x0[:4], countBytes)
		dec.DecodeUint16s(x0)
	*/
	x0 := []uint16{0xff, 0xffff, 0xabcd, 0xdabc}
	buf1 := new(bytes.Buffer)
	n1, err1 := new(int), new(error)
	legacy.WriteUint16s(x0, buf1, n1, err1)
	b1 := buf1.Bytes()
	b2 := pure.EncodeUint16s(x0)
	if !bytes.Equal(b1, b2) {
		t.Fatalf("Bytes do not match for %#v and %#v", b1, b2)
	}
	b3, n3, err3 := dec.DecodeUint16s(b1)
	for i := range b3 {
		if x0[i] != b3[i] {
			t.Fatalf("Decoded Uint16 do not match for %#v and %#v", b3, x0)
		}
	}
	if n3 != *n1 {
		t.Fatalf("Decoded byte count is not correct: %d and %d", n3, *n1)
	}
	assert.Nil(t, err3)
}

func TestUint32(t *testing.T) {
	totry := [5]uint32{0, 1, 0x7FFFFFFE, 0x7FFFFFFF, 0xFFFFFFFF}
	for i := 0; i < len(totry); i += 1 {
		x0 := uint32(totry[i])
		buf1 := new(bytes.Buffer)
		n1, err1 := new(int), new(error)
		legacy.WriteUint32(x0, buf1, n1, err1)
		b1 := buf1.Bytes()
		b2 := pure.EncodeUint32(x0)
		if !bytes.Equal(b1, b2) {
			t.Fatalf("Bytes do not match for %#v and %#v", b1, b2)
		}
		b3, n3, err3 := dec.DecodeUint32(b1)
		if b3 != x0 {
			t.Fatalf("Decoded Uint32 do not match for %#v and %#v", b3, x0)
		}
		if n3 != *n1 {
			t.Fatalf("Decoded byte count is not correct: %d and %d", n3, *n1)
		}
		assert.Nil(t, err3)
	}
}

func TestUint64(t *testing.T) {
	totry := [5]uint64{0, 1, 0x7FFFFFFFFFFFFFFE, 0x7FFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF}
	for i := 0; i < len(totry); i += 1 {
		x0 := uint64(totry[i])
		buf1 := new(bytes.Buffer)
		n1, err1 := new(int), new(error)
		legacy.WriteUint64(x0, buf1, n1, err1)
		b1 := buf1.Bytes()
		b2 := pure.EncodeUint64(x0)
		if !bytes.Equal(b1, b2) {
			t.Fatalf("Bytes do not match for %#v and %#v", b1, b2)
		}
		b3, n3, err3 := dec.DecodeUint64(b1)
		if b3 != x0 {
			t.Fatalf("Decoded Uint64 do not match for %#v and %#v", b3, x0)
		}
		if n3 != *n1 {
			t.Fatalf("Decoded byte count is not correct: %d and %d", n3, *n1)
		}
		assert.Nil(t, err3)
	}
}

func TestInt8(t *testing.T) {
	for i := -0x80; i < 0x80; i += 1 {
		x0 := int8(i)
		buf1 := new(bytes.Buffer)
		n1, err1 := new(int), new(error)
		legacy.WriteInt8(x0, buf1, n1, err1)
		b1 := buf1.Bytes()
		b2 := pure.EncodeInt8(x0)
		if !bytes.Equal(b1, b2) {
			t.Fatalf("Bytes do not match for %#v and %#v", b1, b2)
		}
		b3, n3, err3 := dec.DecodeInt8(b1)
		if b3 != x0 {
			t.Fatalf("Decoded Int8 do not match for %#v and %#v", b3, x0)
		}
		if n3 != *n1 {
			t.Fatalf("Decoded byte count is not correct: %d and %d", n3, *n1)
		}
		assert.Nil(t, err3)
	}
}

func TestInt16(t *testing.T) {
	for i := -0x8000; i < 0x8000; i += 1 {
		x0 := int16(i)
		buf1 := new(bytes.Buffer)
		n1, err1 := new(int), new(error)
		legacy.WriteInt16(x0, buf1, n1, err1)
		b1 := buf1.Bytes()
		b2 := pure.EncodeInt16(x0)
		if !bytes.Equal(b1, b2) {
			t.Fatalf("Bytes do not match for %#v and %#v", b1, b2)
		}
		b3, n3, err3 := dec.DecodeInt16(b1)
		if b3 != x0 {
			t.Fatalf("Decoded Int16 do not match for %#v and %#v", b3, x0)
		}
		if n3 != *n1 {
			t.Fatalf("Decoded byte count is not correct: %d and %d", n3, *n1)
		}
		assert.Nil(t, err3)
	}
}

func TestInt32(t *testing.T) {
	totry := [7]int32{-0x80000000, -0x7FFFFFFF, -1, 0, 1,
		0x7FFFFFFE, 0x7FFFFFFF}
	for i := 0; i < len(totry); i += 1 {
		x0 := int32(totry[i])
		buf1 := new(bytes.Buffer)
		n1, err1 := new(int), new(error)
		legacy.WriteInt32(x0, buf1, n1, err1)
		b1 := buf1.Bytes()
		b2 := pure.EncodeInt32(x0)
		if !bytes.Equal(b1, b2) {
			t.Fatalf("Bytes do not match for %#v and %#v", b1, b2)
		}
		b3, n3, err3 := dec.DecodeInt32(b1)
		if b3 != x0 {
			t.Fatalf("Decoded Int32 do not match for %#v and %#v", b3, x0)
		}
		if n3 != *n1 {
			t.Fatalf("Decoded byte count is not correct: %d and %d", n3, *n1)
		}
		assert.Nil(t, err3)
	}
}
func TestInt64(t *testing.T) {
	totry := [7]int64{-0x8000000000000000, -0x7FFFFFFFFFFFFFFF, -1, 0, 1,
		0x7FFFFFFFFFFFFFFE, 0x7FFFFFFFFFFFFFFF}
	for i := 0; i < len(totry); i += 1 {
		x0 := int64(totry[i])
		buf1 := new(bytes.Buffer)
		n1, err1 := new(int), new(error)
		legacy.WriteInt64(x0, buf1, n1, err1)
		b1 := buf1.Bytes()
		b2 := pure.EncodeInt64(x0)
		if !bytes.Equal(b1, b2) {
			t.Fatalf("Bytes do not match for %#v and %#v", b1, b2)
		}
		b3, n3, err3 := dec.DecodeInt64(b1)
		if b3 != x0 {
			t.Fatalf("Decoded Int64 do not match for %#v and %#v", b3, x0)
		}
		if n3 != *n1 {
			t.Fatalf("Decoded byte count is not correct: %d and %d", n3, *n1)
		}
		assert.Nil(t, err3)
	}
}
