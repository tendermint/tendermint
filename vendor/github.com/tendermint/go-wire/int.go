package wire

import (
	"encoding/binary"
	"errors"
	"io"
)

// Bool

func WriteBool(b bool, w io.Writer, n *int, err *error) {
	var bb byte
	if b {
		bb = 0x01
	} else {
		bb = 0x00
	}
	WriteTo([]byte{bb}, w, n, err)
}

func ReadBool(r io.Reader, n *int, err *error) bool {
	var buf [1]byte
	ReadFull(buf[:], r, n, err)
	switch buf[0] {
	case 0x00:
		return false
	case 0x01:
		return true
	default:
		setFirstErr(err, errors.New("Invalid bool"))
		return false
	}
}

func PutBool(buf []byte, b bool) {
	if b {
		buf[0] = 0x01
	} else {
		buf[0] = 0x00
	}
}

func GetBool(buf []byte) (bool, error) {
	switch buf[0] {
	case 0x00:
		return false, nil
	case 0x01:
		return true, nil
	default:
		return false, errors.New("Invalid bool")
	}
}

// Byte

func WriteByte(b byte, w io.Writer, n *int, err *error) {
	WriteTo([]byte{b}, w, n, err)
}

func ReadByte(r io.Reader, n *int, err *error) byte {
	var buf [1]byte
	ReadFull(buf[:], r, n, err)
	return buf[0]
}

// Int8

func WriteInt8(i int8, w io.Writer, n *int, err *error) {
	WriteByte(byte(i), w, n, err)
}

func ReadInt8(r io.Reader, n *int, err *error) int8 {
	return int8(ReadByte(r, n, err))
}

// Uint8

func WriteUint8(i uint8, w io.Writer, n *int, err *error) {
	WriteByte(byte(i), w, n, err)
}

func ReadUint8(r io.Reader, n *int, err *error) uint8 {
	return uint8(ReadByte(r, n, err))
}

// Int16

func WriteInt16(i int16, w io.Writer, n *int, err *error) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(i))
	WriteTo(buf[:], w, n, err)
}

func ReadInt16(r io.Reader, n *int, err *error) int16 {
	var buf [2]byte
	ReadFull(buf[:], r, n, err)
	return int16(binary.BigEndian.Uint16(buf[:]))
}

func PutInt16(buf []byte, i int16) {
	binary.BigEndian.PutUint16(buf, uint16(i))
}

func GetInt16(buf []byte) int16 {
	return int16(binary.BigEndian.Uint16(buf))
}

// Uint16

func WriteUint16(i uint16, w io.Writer, n *int, err *error) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(i))
	WriteTo(buf[:], w, n, err)
}

func ReadUint16(r io.Reader, n *int, err *error) uint16 {
	var buf [2]byte
	ReadFull(buf[:], r, n, err)
	return uint16(binary.BigEndian.Uint16(buf[:]))
}

func PutUint16(buf []byte, i uint16) {
	binary.BigEndian.PutUint16(buf, i)
}

func GetUint16(buf []byte) uint16 {
	return binary.BigEndian.Uint16(buf)
}

// []Uint16

func WriteUint16s(iz []uint16, w io.Writer, n *int, err *error) {
	WriteUint32(uint32(len(iz)), w, n, err)
	for _, i := range iz {
		WriteUint16(i, w, n, err)
		if *err != nil {
			return
		}
	}
}

func ReadUint16s(r io.Reader, n *int, err *error) []uint16 {
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

func WriteInt32(i int32, w io.Writer, n *int, err *error) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(i))
	WriteTo(buf[:], w, n, err)
}

func ReadInt32(r io.Reader, n *int, err *error) int32 {
	var buf [4]byte
	ReadFull(buf[:], r, n, err)
	return int32(binary.BigEndian.Uint32(buf[:]))
}

func PutInt32(buf []byte, i int32) {
	binary.BigEndian.PutUint32(buf, uint32(i))
}

func GetInt32(buf []byte) int32 {
	return int32(binary.BigEndian.Uint32(buf))
}

// Uint32

func WriteUint32(i uint32, w io.Writer, n *int, err *error) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(i))
	WriteTo(buf[:], w, n, err)
}

func ReadUint32(r io.Reader, n *int, err *error) uint32 {
	var buf [4]byte
	ReadFull(buf[:], r, n, err)
	return uint32(binary.BigEndian.Uint32(buf[:]))
}

func PutUint32(buf []byte, i uint32) {
	binary.BigEndian.PutUint32(buf, i)
}

func GetUint32(buf []byte) uint32 {
	return binary.BigEndian.Uint32(buf)
}

// Int64

func WriteInt64(i int64, w io.Writer, n *int, err *error) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(i))
	WriteTo(buf[:], w, n, err)
}

func ReadInt64(r io.Reader, n *int, err *error) int64 {
	var buf [8]byte
	ReadFull(buf[:], r, n, err)
	return int64(binary.BigEndian.Uint64(buf[:]))
}

func PutInt64(buf []byte, i int64) {
	binary.BigEndian.PutUint64(buf, uint64(i))
}

func GetInt64(buf []byte) int64 {
	return int64(binary.BigEndian.Uint64(buf))
}

// Uint64

func WriteUint64(i uint64, w io.Writer, n *int, err *error) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(i))
	WriteTo(buf[:], w, n, err)
}

func ReadUint64(r io.Reader, n *int, err *error) uint64 {
	var buf [8]byte
	ReadFull(buf[:], r, n, err)
	return uint64(binary.BigEndian.Uint64(buf[:]))
}

func PutUint64(buf []byte, i uint64) {
	binary.BigEndian.PutUint64(buf, i)
}

func GetUint64(buf []byte) uint64 {
	return binary.BigEndian.Uint64(buf)
}

// Varint

func UvarintSize(i uint64) int {
	return uvarintSize(i) + 1 // The first byte encodes uvarintSize(i)
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

func WriteVarint(i int, w io.Writer, n *int, err *error) {
	var negate = false
	if i < 0 {
		negate = true
		i = -i
	}
	var size = uvarintSize(uint64(i))
	if negate {
		// e.g. 0xF1 for a single negative byte
		WriteUint8(uint8(size+0xF0), w, n, err)
	} else {
		WriteUint8(uint8(size), w, n, err)
	}
	if size > 0 {
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(i))
		WriteTo(buf[(8-size):], w, n, err)
	}
}

func ReadVarint(r io.Reader, n *int, err *error) int {
	var size = ReadUint8(r, n, err)
	var negate = false
	if (size >> 4) == 0xF {
		negate = true
		size = size & 0x0F
	}
	if size > 8 {
		setFirstErr(err, errors.New("Varint overflow"))
		return 0
	}
	if size == 0 {
		if negate {
			setFirstErr(err, errors.New("Varint does not allow negative zero"))
		}
		return 0
	}
	var buf [8]byte
	ReadFull(buf[(8-size):], r, n, err)
	var i = int(binary.BigEndian.Uint64(buf[:]))
	if negate {
		return -i
	} else {
		return i
	}
}

func PutVarint(buf []byte, i int) (n int, err error) {
	var negate = false
	if i < 0 {
		negate = true
		i = -i
	}
	var size = uvarintSize(uint64(i))
	if len(buf) < size+1 {
		return 0, errors.New("Insufficient buffer length")
	}
	if negate {
		// e.g. 0xF1 for a single negative byte
		buf[0] = byte(size + 0xF0)
	} else {
		buf[0] = byte(size)
	}
	if size > 0 {
		var buf2 [8]byte
		binary.BigEndian.PutUint64(buf2[:], uint64(i))
		copy(buf[1:], buf2[(8-size):])
	}
	return size + 1, nil
}

func GetVarint(buf []byte) (i int, n int, err error) {
	if len(buf) == 0 {
		return 0, 0, errors.New("Insufficent buffer length")
	}
	var size = int(buf[0])
	var negate = false
	if (size >> 4) == 0xF {
		negate = true
		size = size & 0x0F
	}
	if size > 8 {
		return 0, 0, errors.New("Varint overflow")
	}
	if size == 0 {
		if negate {
			return 0, 0, errors.New("Varint does not allow negative zero")
		}
		return 0, 1, nil
	}
	if len(buf) < 1+size {
		return 0, 0, errors.New("Insufficient buffer length")
	}
	var buf2 [8]byte
	copy(buf2[(8-size):], buf[1:1+size])
	i = int(binary.BigEndian.Uint64(buf2[:]))
	if negate {
		return -i, size + 1, nil
	} else {
		return i, size + 1, nil
	}
}

// Uvarint

func WriteUvarint(i uint, w io.Writer, n *int, err *error) {
	var size = uvarintSize(uint64(i))
	WriteUint8(uint8(size), w, n, err)
	if size > 0 {
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(i))
		WriteTo(buf[(8-size):], w, n, err)
	}
}

func ReadUvarint(r io.Reader, n *int, err *error) uint {
	var size = ReadUint8(r, n, err)
	if size > 8 {
		setFirstErr(err, errors.New("Uvarint overflow"))
		return 0
	}
	if size == 0 {
		return 0
	}
	var buf [8]byte
	ReadFull(buf[(8-size):], r, n, err)
	return uint(binary.BigEndian.Uint64(buf[:]))
}

func PutUvarint(buf []byte, i uint) (n int, err error) {
	var size = uvarintSize(uint64(i))
	if len(buf) < size+1 {
		return 0, errors.New("Insufficient buffer length")
	}
	buf[0] = byte(size)
	if size > 0 {
		var buf2 [8]byte
		binary.BigEndian.PutUint64(buf2[:], uint64(i))
		copy(buf[1:], buf2[(8-size):])
	}
	return size + 1, nil
}

func GetUvarint(buf []byte) (i uint, n int, err error) {
	if len(buf) == 0 {
		return 0, 0, errors.New("Insufficent buffer length")
	}
	var size = int(buf[0])
	if size > 8 {
		return 0, 0, errors.New("Uvarint overflow")
	}
	if size == 0 {
		return 0, 1, nil
	}
	if len(buf) < 1+size {
		return 0, 0, errors.New("Insufficient buffer length")
	}
	var buf2 [8]byte
	copy(buf2[(8-size):], buf[1:1+size])
	i = uint(binary.BigEndian.Uint64(buf2[:]))
	return i, size + 1, nil
}

func setFirstErr(err *error, newErr error) {
	if *err == nil && newErr != nil {
		*err = newErr
	}
}
