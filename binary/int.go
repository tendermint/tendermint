package binary

import (
	"encoding/binary"
	"errors"
	"io"
)

// Byte

func WriteByte(b byte, w io.Writer, n *int64, err *error) {
	WriteTo([]byte{b}, w, n, err)
}

func ReadByte(r io.Reader, n *int64, err *error) byte {
	buf := make([]byte, 1)
	ReadFull(buf, r, n, err)
	return buf[0]
}

// Int8

func WriteInt8(i int8, w io.Writer, n *int64, err *error) {
	WriteByte(byte(i), w, n, err)
}

func ReadInt8(r io.Reader, n *int64, err *error) int8 {
	return int8(ReadByte(r, n, err))
}

// Uint8

func WriteUint8(i uint8, w io.Writer, n *int64, err *error) {
	WriteByte(byte(i), w, n, err)
}

func ReadUint8(r io.Reader, n *int64, err *error) uint8 {
	return uint8(ReadByte(r, n, err))
}

// Int16

func WriteInt16(i int16, w io.Writer, n *int64, err *error) {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(i))
	*n += 2
	WriteTo(buf, w, n, err)
}

func ReadInt16(r io.Reader, n *int64, err *error) int16 {
	buf := make([]byte, 2)
	ReadFull(buf, r, n, err)
	return int16(binary.LittleEndian.Uint16(buf))
}

// Uint16

func WriteUint16(i uint16, w io.Writer, n *int64, err *error) {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(i))
	*n += 2
	WriteTo(buf, w, n, err)
}

func ReadUint16(r io.Reader, n *int64, err *error) uint16 {
	buf := make([]byte, 2)
	ReadFull(buf, r, n, err)
	return uint16(binary.LittleEndian.Uint16(buf))
}

// []Uint16

func WriteUint16s(iz []uint16, w io.Writer, n *int64, err *error) {
	WriteUint32(uint32(len(iz)), w, n, err)
	for _, i := range iz {
		WriteUint16(i, w, n, err)
		if *err != nil {
			return
		}
	}
}

func ReadUint16s(r io.Reader, n *int64, err *error) []uint16 {
	length := ReadUint32(r, n, err)
	if *err != nil {
		return nil
	}
	iz := make([]uint16, length)
	for j := uint32(0); j < length; j++ {
		ii := ReadUint16(r, n, err)
		if *err != nil {
			return nil
		}
		iz[j] = ii
	}
	return iz
}

// Int32

func WriteInt32(i int32, w io.Writer, n *int64, err *error) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(i))
	*n += 4
	WriteTo(buf, w, n, err)
}

func ReadInt32(r io.Reader, n *int64, err *error) int32 {
	buf := make([]byte, 4)
	ReadFull(buf, r, n, err)
	return int32(binary.LittleEndian.Uint32(buf))
}

// Uint32

func WriteUint32(i uint32, w io.Writer, n *int64, err *error) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(i))
	*n += 4
	WriteTo(buf, w, n, err)
}

func ReadUint32(r io.Reader, n *int64, err *error) uint32 {
	buf := make([]byte, 4)
	ReadFull(buf, r, n, err)
	return uint32(binary.LittleEndian.Uint32(buf))
}

// Int64

func WriteInt64(i int64, w io.Writer, n *int64, err *error) {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(i))
	*n += 8
	WriteTo(buf, w, n, err)
}

func ReadInt64(r io.Reader, n *int64, err *error) int64 {
	buf := make([]byte, 8)
	ReadFull(buf, r, n, err)
	return int64(binary.LittleEndian.Uint64(buf))
}

// Uint64

func WriteUint64(i uint64, w io.Writer, n *int64, err *error) {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(i))
	*n += 8
	WriteTo(buf, w, n, err)
}

func ReadUint64(r io.Reader, n *int64, err *error) uint64 {
	buf := make([]byte, 8)
	ReadFull(buf, r, n, err)
	return uint64(binary.LittleEndian.Uint64(buf))
}

// Varint

func WriteVarint(i int, w io.Writer, n *int64, err *error) {
	buf := make([]byte, binary.MaxVarintLen64)
	n_ := int64(binary.PutVarint(buf, int64(i)))
	*n += n_
	WriteTo(buf[:n_], w, n, err)
}

func ReadVarint(r io.Reader, n *int64, err *error) int {
	res, n_, err_ := readVarint(r)
	*n += n_
	*err = err_
	return int(res)
}

// Uvarint

func WriteUvarint(i uint, w io.Writer, n *int64, err *error) {
	buf := make([]byte, binary.MaxVarintLen64)
	n_ := int64(binary.PutUvarint(buf, uint64(i)))
	*n += n_
	WriteTo(buf[:n_], w, n, err)
}

func ReadUvarint(r io.Reader, n *int64, err *error) uint {
	res, n_, err_ := readUvarint(r)
	*n += n_
	*err = err_
	return uint(res)
}

//-----------------------------------------------------------------------------

var overflow = errors.New("binary: varint overflows a 64-bit integer")

// Modified to return number of bytes read, from
// http://golang.org/src/pkg/encoding/binary/varint.go?s=3652:3699#L116
func readUvarint(r io.Reader) (uint64, int64, error) {
	var x uint64
	var s uint
	var buf = make([]byte, 1)
	for i := 0; ; i++ {
		for {
			n, err := r.Read(buf)
			if err != nil {
				return x, int64(i), err
			}
			if n > 0 {
				break
			}
		}
		b := buf[0]
		if b < 0x80 {
			if i > 9 || i == 9 && b > 1 {
				return x, int64(i), overflow
			}
			return x | uint64(b)<<s, int64(i), nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
}

// Modified to return number of bytes read, from
// http://golang.org/src/pkg/encoding/binary/varint.go?s=3652:3699#L116
func readVarint(r io.Reader) (int64, int64, error) {
	ux, n, err := readUvarint(r) // ok to continue in presence of error
	x := int64(ux >> 1)
	if ux&1 != 0 {
		x = ^x
	}
	return x, n, err
}
