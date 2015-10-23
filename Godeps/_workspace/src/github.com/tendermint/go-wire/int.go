package wire

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
	binary.BigEndian.PutUint16(buf, uint16(i))
	*n += 2
	WriteTo(buf, w, n, err)
}

func ReadInt16(r io.Reader, n *int64, err *error) int16 {
	buf := make([]byte, 2)
	ReadFull(buf, r, n, err)
	return int16(binary.BigEndian.Uint16(buf))
}

// Uint16

func WriteUint16(i uint16, w io.Writer, n *int64, err *error) {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(i))
	*n += 2
	WriteTo(buf, w, n, err)
}

func ReadUint16(r io.Reader, n *int64, err *error) uint16 {
	buf := make([]byte, 2)
	ReadFull(buf, r, n, err)
	return uint16(binary.BigEndian.Uint16(buf))
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
	binary.BigEndian.PutUint32(buf, uint32(i))
	*n += 4
	WriteTo(buf, w, n, err)
}

func ReadInt32(r io.Reader, n *int64, err *error) int32 {
	buf := make([]byte, 4)
	ReadFull(buf, r, n, err)
	return int32(binary.BigEndian.Uint32(buf))
}

// Uint32

func WriteUint32(i uint32, w io.Writer, n *int64, err *error) {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(i))
	*n += 4
	WriteTo(buf, w, n, err)
}

func ReadUint32(r io.Reader, n *int64, err *error) uint32 {
	buf := make([]byte, 4)
	ReadFull(buf, r, n, err)
	return uint32(binary.BigEndian.Uint32(buf))
}

// Int64

func WriteInt64(i int64, w io.Writer, n *int64, err *error) {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(i))
	*n += 8
	WriteTo(buf, w, n, err)
}

func ReadInt64(r io.Reader, n *int64, err *error) int64 {
	buf := make([]byte, 8)
	ReadFull(buf, r, n, err)
	return int64(binary.BigEndian.Uint64(buf))
}

// Uint64

func WriteUint64(i uint64, w io.Writer, n *int64, err *error) {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(i))
	*n += 8
	WriteTo(buf, w, n, err)
}

func ReadUint64(r io.Reader, n *int64, err *error) uint64 {
	buf := make([]byte, 8)
	ReadFull(buf, r, n, err)
	return uint64(binary.BigEndian.Uint64(buf))
}

// Varint

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

func WriteVarint(i int, w io.Writer, n *int64, err *error) {
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
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(i))
		WriteTo(buf[(8-size):], w, n, err)
	}
	*n += int64(1 + size)
}

func ReadVarint(r io.Reader, n *int64, err *error) int {
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
	buf := make([]byte, 8)
	ReadFull(buf[(8-size):], r, n, err)
	*n += int64(1 + size)
	var i = int(binary.BigEndian.Uint64(buf))
	if negate {
		return -i
	} else {
		return i
	}
}

// Uvarint

func WriteUvarint(i uint, w io.Writer, n *int64, err *error) {
	var size = uvarintSize(uint64(i))
	WriteUint8(uint8(size), w, n, err)
	if size > 0 {
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(i))
		WriteTo(buf[(8-size):], w, n, err)
	}
	*n += int64(1 + size)
}

func ReadUvarint(r io.Reader, n *int64, err *error) uint {
	var size = ReadUint8(r, n, err)
	if size > 8 {
		setFirstErr(err, errors.New("Uvarint overflow"))
		return 0
	}
	if size == 0 {
		return 0
	}
	buf := make([]byte, 8)
	ReadFull(buf[(8-size):], r, n, err)
	*n += int64(1 + size)
	return uint(binary.BigEndian.Uint64(buf))
}

func setFirstErr(err *error, newErr error) {
	if *err == nil && newErr != nil {
		*err = newErr
	}
}
