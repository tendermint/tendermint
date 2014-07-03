package binary

import (
	"encoding/binary"
	"io"
)

type Byte byte
type Int8 int8
type UInt8 uint8
type Int16 int16
type UInt16 uint16
type Int32 int32
type UInt32 uint32
type Int64 int64
type UInt64 uint64
type Int int
type UInt uint

// Byte

func (self Byte) Equals(other Binary) bool {
	return self == other
}

func (self Byte) Less(other Binary) bool {
	if o, ok := other.(Byte); ok {
		return self < o
	} else {
		panic("Cannot compare unequal types")
	}
}

func (self Byte) ByteSize() int {
	return 1
}

func (self Byte) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write([]byte{byte(self)})
	return int64(n), err
}

func ReadByteSafe(r io.Reader) (Byte, error) {
	buf := [1]byte{0}
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return Byte(buf[0]), nil
}

func ReadByte(r io.Reader) Byte {
	b, err := ReadByteSafe(r)
	if err != nil {
		panic(err)
	}
	return b
}

// Int8

func (self Int8) Equals(other Binary) bool {
	return self == other
}

func (self Int8) Less(other Binary) bool {
	if o, ok := other.(Int8); ok {
		return self < o
	} else {
		panic("Cannot compare unequal types")
	}
}

func (self Int8) ByteSize() int {
	return 1
}

func (self Int8) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write([]byte{byte(self)})
	return int64(n), err
}

func ReadInt8Safe(r io.Reader) (Int8, error) {
	buf := [1]byte{0}
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return Int8(0), err
	}
	return Int8(buf[0]), nil
}

func ReadInt8(r io.Reader) Int8 {
	b, err := ReadInt8Safe(r)
	if err != nil {
		panic(err)
	}
	return b
}

// UInt8

func (self UInt8) Equals(other Binary) bool {
	return self == other
}

func (self UInt8) Less(other Binary) bool {
	if o, ok := other.(UInt8); ok {
		return self < o
	} else {
		panic("Cannot compare unequal types")
	}
}

func (self UInt8) ByteSize() int {
	return 1
}

func (self UInt8) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write([]byte{byte(self)})
	return int64(n), err
}

func ReadUInt8Safe(r io.Reader) (UInt8, error) {
	buf := [1]byte{0}
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return UInt8(0), err
	}
	return UInt8(buf[0]), nil
}

func ReadUInt8(r io.Reader) UInt8 {
	b, err := ReadUInt8Safe(r)
	if err != nil {
		panic(err)
	}
	return b
}

// Int16

func (self Int16) Equals(other Binary) bool {
	return self == other
}

func (self Int16) Less(other Binary) bool {
	if o, ok := other.(Int16); ok {
		return self < o
	} else {
		panic("Cannot compare unequal types")
	}
}

func (self Int16) ByteSize() int {
	return 2
}

func (self Int16) WriteTo(w io.Writer) (int64, error) {
	buf := []byte{0, 0}
	binary.LittleEndian.PutUint16(buf, uint16(self))
	n, err := w.Write(buf)
	return int64(n), err
}

func ReadInt16Safe(r io.Reader) (Int16, error) {
	buf := [2]byte{0}
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return Int16(0), err
	}
	return Int16(binary.LittleEndian.Uint16(buf[:])), nil
}

func ReadInt16(r io.Reader) Int16 {
	b, err := ReadInt16Safe(r)
	if err != nil {
		panic(err)
	}
	return b
}

// UInt16

func (self UInt16) Equals(other Binary) bool {
	return self == other
}

func (self UInt16) Less(other Binary) bool {
	if o, ok := other.(UInt16); ok {
		return self < o
	} else {
		panic("Cannot compare unequal types")
	}
}

func (self UInt16) ByteSize() int {
	return 2
}

func (self UInt16) WriteTo(w io.Writer) (int64, error) {
	buf := []byte{0, 0}
	binary.LittleEndian.PutUint16(buf, uint16(self))
	n, err := w.Write(buf)
	return int64(n), err
}

func ReadUInt16Safe(r io.Reader) (UInt16, error) {
	buf := [2]byte{0}
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return UInt16(0), err
	}
	return UInt16(binary.LittleEndian.Uint16(buf[:])), nil
}

func ReadUInt16(r io.Reader) UInt16 {
	b, err := ReadUInt16Safe(r)
	if err != nil {
		panic(err)
	}
	return b
}

// Int32

func (self Int32) Equals(other Binary) bool {
	return self == other
}

func (self Int32) Less(other Binary) bool {
	if o, ok := other.(Int32); ok {
		return self < o
	} else {
		panic("Cannot compare unequal types")
	}
}

func (self Int32) ByteSize() int {
	return 4
}

func (self Int32) WriteTo(w io.Writer) (int64, error) {
	buf := []byte{0, 0, 0, 0}
	binary.LittleEndian.PutUint32(buf, uint32(self))
	n, err := w.Write(buf)
	return int64(n), err
}

func ReadInt32Safe(r io.Reader) (Int32, error) {
	buf := [4]byte{0}
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return Int32(0), err
	}
	return Int32(binary.LittleEndian.Uint32(buf[:])), nil
}

func ReadInt32(r io.Reader) Int32 {
	b, err := ReadInt32Safe(r)
	if err != nil {
		panic(err)
	}
	return b
}

// UInt32

func (self UInt32) Equals(other Binary) bool {
	return self == other
}

func (self UInt32) Less(other Binary) bool {
	if o, ok := other.(UInt32); ok {
		return self < o
	} else {
		panic("Cannot compare unequal types")
	}
}

func (self UInt32) ByteSize() int {
	return 4
}

func (self UInt32) WriteTo(w io.Writer) (int64, error) {
	buf := []byte{0, 0, 0, 0}
	binary.LittleEndian.PutUint32(buf, uint32(self))
	n, err := w.Write(buf)
	return int64(n), err
}

func ReadUInt32Safe(r io.Reader) (UInt32, error) {
	buf := [4]byte{0}
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return UInt32(0), err
	}
	return UInt32(binary.LittleEndian.Uint32(buf[:])), nil
}

func ReadUInt32(r io.Reader) UInt32 {
	b, err := ReadUInt32Safe(r)
	if err != nil {
		panic(err)
	}
	return b
}

// Int64

func (self Int64) Equals(other Binary) bool {
	return self == other
}

func (self Int64) Less(other Binary) bool {
	if o, ok := other.(Int64); ok {
		return self < o
	} else {
		panic("Cannot compare unequal types")
	}
}

func (self Int64) ByteSize() int {
	return 8
}

func (self Int64) WriteTo(w io.Writer) (int64, error) {
	buf := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint64(buf, uint64(self))
	n, err := w.Write(buf)
	return int64(n), err
}

func ReadInt64Safe(r io.Reader) (Int64, error) {
	buf := [8]byte{0}
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return Int64(0), err
	}
	return Int64(binary.LittleEndian.Uint64(buf[:])), nil
}

func ReadInt64(r io.Reader) Int64 {
	b, err := ReadInt64Safe(r)
	if err != nil {
		panic(err)
	}
	return b
}

// UInt64

func (self UInt64) Equals(other Binary) bool {
	return self == other
}

func (self UInt64) Less(other Binary) bool {
	if o, ok := other.(UInt64); ok {
		return self < o
	} else {
		panic("Cannot compare unequal types")
	}
}

func (self UInt64) ByteSize() int {
	return 8
}

func (self UInt64) WriteTo(w io.Writer) (int64, error) {
	buf := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint64(buf, uint64(self))
	n, err := w.Write(buf)
	return int64(n), err
}

func ReadUInt64Safe(r io.Reader) (UInt64, error) {
	buf := [8]byte{0}
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return UInt64(0), err
	}
	return UInt64(binary.LittleEndian.Uint64(buf[:])), nil
}

func ReadUInt64(r io.Reader) UInt64 {
	b, err := ReadUInt64Safe(r)
	if err != nil {
		panic(err)
	}
	return b
}

// Int

func (self Int) Equals(other Binary) bool {
	return self == other
}

func (self Int) Less(other Binary) bool {
	if o, ok := other.(Int); ok {
		return self < o
	} else {
		panic("Cannot compare unequal types")
	}
}

func (self Int) ByteSize() int {
	return 8
}

func (self Int) WriteTo(w io.Writer) (int64, error) {
	buf := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint64(buf, uint64(self))
	n, err := w.Write(buf)
	return int64(n), err
}

func ReadInt(r io.Reader) Int {
	buf := [8]byte{0}
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		panic(err)
	}
	return Int(binary.LittleEndian.Uint64(buf[:]))
}

// UInt

func (self UInt) Equals(other Binary) bool {
	return self == other
}

func (self UInt) Less(other Binary) bool {
	if o, ok := other.(UInt); ok {
		return self < o
	} else {
		panic("Cannot compare unequal types")
	}
}

func (self UInt) ByteSize() int {
	return 8
}

func (self UInt) WriteTo(w io.Writer) (int64, error) {
	buf := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint64(buf, uint64(self))
	n, err := w.Write(buf)
	return int64(n), err
}

func ReadUInt(r io.Reader) UInt {
	buf := [8]byte{0}
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		panic(err)
	}
	return UInt(binary.LittleEndian.Uint64(buf[:]))
}
