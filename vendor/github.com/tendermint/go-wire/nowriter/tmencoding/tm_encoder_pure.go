package tmencoding

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"github.com/tendermint/go-wire/nowriter/tmlegacy"
	"github.com/tendermint/go-wire/nowriter/tmvarint"
	cmn "github.com/tendermint/tmlibs/common"
	"math"
	"time"
)

// Implementation of the TMEncoder interface.
type TMEncoderPure struct {
}

var _ TMEncoder = TMEncoderPure{}
var legacy tmlegacy.TMEncoderLegacy
var v0varint tmvarint.TMVarint = tmvarint.TMVarintV0{}

func (e TMEncoderPure) EncodeBool(b bool) []byte {
	var bb byte
	if b {
		bb = 0x01
	} else {
		bb = 0x00
	}
	return []byte{bb}
}

func (e TMEncoderPure) EncodeFloat32(f float32) []byte {
	return e.EncodeUint32(math.Float32bits(f))
}

func (e TMEncoderPure) EncodeFloat64(f float64) []byte {
	return e.EncodeUint64(math.Float64bits(f))
}

func (e TMEncoderPure) EncodeInt8(i int8) []byte {
	return e.EncodeOctet(byte(i))
}

func (e TMEncoderPure) EncodeInt16(i int16) []byte {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(i))
	return buf[:]
}

func (e TMEncoderPure) EncodeInt32(i int32) []byte {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(i))
	return buf[:]
}

func (e TMEncoderPure) EncodeInt64(i int64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(i))
	return buf[:]
}

func (e TMEncoderPure) EncodeOctet(b byte) []byte {
	return []byte{b}
}

// for orthogonality only
func (e TMEncoderPure) EncodeOctets(b []byte) []byte {
	arr := make([]byte, len(b))
	copy(arr, b)
	return arr
}

func (e TMEncoderPure) EncodeTime(t time.Time) []byte {
	nanosecs := t.UnixNano()
	millisecs := nanosecs / 1000000
	if nanosecs < 0 {
		cmn.PanicSanity("can't encode times below 1970")
	}
	return e.EncodeInt64(millisecs * 1000000)
}

func (e TMEncoderPure) EncodeUint8(i uint8) []byte {
	return e.EncodeOctet(byte(i))
}

func (e TMEncoderPure) EncodeUint16(i uint16) []byte {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(i))
	return buf[:]
}

func (e TMEncoderPure) EncodeUint16s(iz []uint16) []byte {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	var inst_n int
	n := &inst_n
	var inst_err error
	err := &inst_err

	legacy.WriteUint32(uint32(len(iz)), w, n, err)
	for _, i := range iz {
		legacy.WriteUint16(i, w, n, err)
		if *err != nil {
			return nil
		}
	}

	return b.Bytes()
}

func (e TMEncoderPure) EncodeUint32(i uint32) []byte {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(i))
	return buf[:]
}

func (e TMEncoderPure) EncodeUint64(i uint64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(i))
	return buf[:]
}

func (e TMEncoderPure) EncodeUvarint(i uint) []byte {
	return v0varint.EncodeUvarint(i)
}

func (e TMEncoderPure) EncodeVarint(i int) []byte {
	return v0varint.EncodeVarint(i)
}
