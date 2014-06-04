package merkle

import (
    "io"
    "encoding/binary"
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

func (self Byte) Less(other Key) bool {
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

func ReadByte(bytes []byte) Byte {
    return Byte(bytes[0])
}


// Int8

func (self Int8) Equals(other Binary) bool {
    return self == other
}

func (self Int8) Less(other Key) bool {
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

func ReadInt8(bytes []byte) Int8 {
    return Int8(bytes[0])
}


// UInt8

func (self UInt8) Equals(other Binary) bool {
    return self == other
}

func (self UInt8) Less(other Key) bool {
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

func ReadUInt8(bytes []byte) UInt8 {
    return UInt8(bytes[0])
}


// Int16

func (self Int16) Equals(other Binary) bool {
    return self == other
}

func (self Int16) Less(other Key) bool {
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
    err := binary.Write(w, binary.LittleEndian, int16(self))
    return 2, err
}

func ReadInt16(bytes []byte) Int16 {
    return Int16(binary.LittleEndian.Uint16(bytes))
}


// UInt16

func (self UInt16) Equals(other Binary) bool {
    return self == other
}

func (self UInt16) Less(other Key) bool {
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
    err := binary.Write(w, binary.LittleEndian, uint16(self))
    return 2, err
}

func ReadUInt16(bytes []byte) UInt16 {
    return UInt16(binary.LittleEndian.Uint16(bytes))
}


// Int32

func (self Int32) Equals(other Binary) bool {
    return self == other
}

func (self Int32) Less(other Key) bool {
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
    err := binary.Write(w, binary.LittleEndian, int32(self))
    return 4, err
}

func ReadInt32(bytes []byte) Int32 {
    return Int32(binary.LittleEndian.Uint32(bytes))
}


// UInt32

func (self UInt32) Equals(other Binary) bool {
    return self == other
}

func (self UInt32) Less(other Key) bool {
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
    err := binary.Write(w, binary.LittleEndian, uint32(self))
    return 4, err
}

func ReadUInt32(bytes []byte) UInt32 {
    return UInt32(binary.LittleEndian.Uint32(bytes))
}


// Int64

func (self Int64) Equals(other Binary) bool {
    return self == other
}

func (self Int64) Less(other Key) bool {
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
    err := binary.Write(w, binary.LittleEndian, int64(self))
    return 8, err
}

func ReadInt64(bytes []byte) Int64 {
    return Int64(binary.LittleEndian.Uint64(bytes))
}


// UInt64

func (self UInt64) Equals(other Binary) bool {
    return self == other
}

func (self UInt64) Less(other Key) bool {
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
    err := binary.Write(w, binary.LittleEndian, uint64(self))
    return 8, err
}

func ReadUInt64(bytes []byte) UInt64 {
    return UInt64(binary.LittleEndian.Uint64(bytes))
}


// Int

func (self Int) Equals(other Binary) bool {
    return self == other
}

func (self Int) Less(other Key) bool {
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
    err := binary.Write(w, binary.LittleEndian, int64(self))
    return 8, err
}

func ReadInt(bytes []byte) Int {
    return Int(binary.LittleEndian.Uint64(bytes))
}

// UInt

func (self UInt) Equals(other Binary) bool {
    return self == other
}

func (self UInt) Less(other Key) bool {
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
    err := binary.Write(w, binary.LittleEndian, uint64(self))
    return 8, err
}

func ReadUInt(bytes []byte) UInt {
    return UInt(binary.LittleEndian.Uint64(bytes))
}
