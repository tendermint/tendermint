package binary

import (
	"encoding/binary"
	"io"
)

// Byte

func WriteByte(w io.Writer, b byte, n *int64, err *error) {
	WriteTo(w, []byte{b}, n, err)
}

func ReadByte(r io.Reader, n *int64, err *error) byte {
	buf := make([]byte, 1)
	ReadFull(r, buf, n, err)
	return buf[0]
}

// Int8

func WriteInt8(w io.Writer, i int8, n *int64, err *error) {
	WriteByte(w, byte(i), n, err)
}

func ReadInt8(r io.Reader, n *int64, err *error) int8 {
	return int8(ReadByte(r, n, err))
}

// UInt8

func WriteUInt8(w io.Writer, i uint8, n *int64, err *error) {
	WriteByte(w, byte(i), n, err)
}

func ReadUInt8(r io.Reader, n *int64, err *error) uint8 {
	return uint8(ReadByte(r, n, err))
}

// Int16

func WriteInt16(w io.Writer, i int16, n *int64, err *error) {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(i))
	WriteTo(w, buf, n, err)
}

func ReadInt16(r io.Reader, n *int64, err *error) int16 {
	buf := make([]byte, 2)
	ReadFull(r, buf, n, err)
	return int16(binary.LittleEndian.Uint16(buf))
}

// UInt16

func WriteUInt16(w io.Writer, i uint16, n *int64, err *error) {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(i))
	WriteTo(w, buf, n, err)
}

func ReadUInt16(r io.Reader, n *int64, err *error) uint16 {
	buf := make([]byte, 2)
	ReadFull(r, buf, n, err)
	return uint16(binary.LittleEndian.Uint16(buf))
}

// Int32

func WriteInt32(w io.Writer, i int32, n *int64, err *error) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(i))
	WriteTo(w, buf, n, err)
}

func ReadInt32(r io.Reader, n *int64, err *error) int32 {
	buf := make([]byte, 4)
	ReadFull(r, buf, n, err)
	return int32(binary.LittleEndian.Uint32(buf))
}

// UInt32

func WriteUInt32(w io.Writer, i uint32, n *int64, err *error) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(i))
	WriteTo(w, buf, n, err)
}

func ReadUInt32(r io.Reader, n *int64, err *error) uint32 {
	buf := make([]byte, 4)
	ReadFull(r, buf, n, err)
	return uint32(binary.LittleEndian.Uint32(buf))
}

// Int64

func WriteInt64(w io.Writer, i int64, n *int64, err *error) {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(i))
	WriteTo(w, buf, n, err)
}

func ReadInt64(r io.Reader, n *int64, err *error) int64 {
	buf := make([]byte, 8)
	ReadFull(r, buf, n, err)
	return int64(binary.LittleEndian.Uint64(buf))
}

// UInt64

func WriteUInt64(w io.Writer, i uint64, n *int64, err *error) {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(i))
	WriteTo(w, buf, n, err)
}

func ReadUInt64(r io.Reader, n *int64, err *error) uint64 {
	buf := make([]byte, 8)
	ReadFull(r, buf, n, err)
	return uint64(binary.LittleEndian.Uint64(buf))
}
