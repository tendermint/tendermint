package tmlegacy

import (
	"encoding/binary"
	cmn "github.com/tendermint/tmlibs/common"
	"io"
	"math"
	"time"
)

// Implementation of the legacy (`TMEncoderFastIOWriter`) interface
type TMEncoderLegacy struct {
}

var Legacy *TMEncoderLegacy = &TMEncoderLegacy{} // convenience

// Does not use builder pattern to encourage migration away from this struct
func (e *TMEncoderLegacy) WriteBool(b bool, w io.Writer, n *int, err *error) {
	var bb byte
	if b {
		bb = 0x01
	} else {
		bb = 0x00
	}
	e.WriteTo([]byte{bb}, w, n, err)
}

func (e *TMEncoderLegacy) WriteFloat32(f float32, w io.Writer, n *int, err *error) {
	e.WriteUint32(math.Float32bits(f), w, n, err)
}

func (e *TMEncoderLegacy) WriteFloat64(f float64, w io.Writer, n *int, err *error) {
	e.WriteUint64(math.Float64bits(f), w, n, err)
}

func (e *TMEncoderLegacy) WriteInt8(i int8, w io.Writer, n *int, err *error) {
	e.WriteOctet(byte(i), w, n, err)
}

func (e *TMEncoderLegacy) WriteInt16(i int16, w io.Writer, n *int, err *error) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(i))
	e.WriteTo(buf[:], w, n, err)
}

func (e *TMEncoderLegacy) WriteInt32(i int32, w io.Writer, n *int, err *error) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(i))
	e.WriteTo(buf[:], w, n, err)
}
func (e *TMEncoderLegacy) WriteInt64(i int64, w io.Writer, n *int, err *error) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(i))
	e.WriteTo(buf[:], w, n, err)
}

func (e *TMEncoderLegacy) WriteOctet(b byte, w io.Writer, n *int, err *error) {
	e.WriteTo([]byte{b}, w, n, err)
}

func (e *TMEncoderLegacy) WriteTime(t time.Time, w io.Writer, n *int, err *error) {
	nanosecs := t.UnixNano()
	millisecs := nanosecs / 1000000
	if nanosecs < 0 {
		cmn.PanicSanity("can't encode times below 1970")
	} else {
		e.WriteInt64(millisecs*1000000, w, n, err)
	}
}

func (e *TMEncoderLegacy) WriteUint8(i uint8, w io.Writer, n *int, err *error) {
	e.WriteOctet(byte(i), w, n, err)
}

func (e *TMEncoderLegacy) WriteUint16(i uint16, w io.Writer, n *int, err *error) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(i))
	e.WriteTo(buf[:], w, n, err)
}

func (e *TMEncoderLegacy) WriteUint16s(iz []uint16, w io.Writer, n *int, err *error) {
	e.WriteUint32(uint32(len(iz)), w, n, err)
	for _, i := range iz {
		e.WriteUint16(i, w, n, err)
		if *err != nil {
			return
		}
	}
}
func (e *TMEncoderLegacy) WriteUint32(i uint32, w io.Writer, n *int, err *error) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(i))
	e.WriteTo(buf[:], w, n, err)
}

func (e *TMEncoderLegacy) WriteUint64(i uint64, w io.Writer, n *int, err *error) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(i))
	e.WriteTo(buf[:], w, n, err)
}

func (e *TMEncoderLegacy) WriteUvarint(i uint, w io.Writer, n *int, err *error) {
	var size = uvarintSize(uint64(i))
	e.WriteUint8(uint8(size), w, n, err)
	if size > 0 {
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(i))
		e.WriteTo(buf[(8-size):], w, n, err)
	}
}

func (e *TMEncoderLegacy) WriteVarint(i int, w io.Writer, n *int, err *error) {
	var negate = false
	if i < 0 {
		negate = true
		i = -i
	}
	var size = uvarintSize(uint64(i))
	if negate {
		// e.g. 0xF1 for a single negative byte
		e.WriteUint8(uint8(size+0xF0), w, n, err)
	} else {
		e.WriteUint8(uint8(size), w, n, err)
	}
	if size > 0 {
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(i))
		e.WriteTo(buf[(8-size):], w, n, err)
	}
}

// Write all of bz to w
// Increment n and set err accordingly.
func (e *TMEncoderLegacy) WriteTo(bz []byte, w io.Writer, n *int, err *error) {
	if *err != nil {
		return
	}
	n_, err_ := w.Write(bz)
	*n += n_
	*err = err_
}

func uvarintSize(i uint64) int {
	if i == 0 {
		return 0
	}
	if i < 1<<8 {
		return 1
	}
	if i < 1<<16 {
		return 2
	}
	if i < 1<<24 {
		return 3
	}
	if i < 1<<32 {
		return 4
	}
	if i < 1<<40 {
		return 5
	}
	if i < 1<<48 {
		return 6
	}
	if i < 1<<56 {
		return 7
	}
	return 8
}

func (e *TMEncoderLegacy) WriteOctetSlice(bz []byte, w io.Writer, n *int, err *error) {
	e.WriteVarint(len(bz), w, n, err)
	if len(bz) > 0 {
		e.WriteTo(bz, w, n, err)
	}
}
