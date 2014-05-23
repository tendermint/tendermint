package merkle

import (
    "encoding/binary"
)

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


// Int8

func (self Int8) Equals(other Key) bool {
    if o, ok := other.(Int8); ok {
        return self == o
    } else {
        return false
    }
}

func (self Int8) Less(other Key) bool {
    if o, ok := other.(Int8); ok {
        return self < o
    } else {
        return false
    }
}

func (self Int8) ByteSize() int {
    return 1
}

func (self Int8) SaveTo(b []byte) int {
    if cap(b) < 1 { panic("buf too small") }
    b[0] = byte(self)
    return 1
}

func LoadInt8(bytes []byte) Int8 {
    return Int8(bytes[0])
}


// UInt8

func (self UInt8) Equals(other Key) bool {
    if o, ok := other.(UInt8); ok {
        return self == o
    } else {
        return false
    }
}

func (self UInt8) Less(other Key) bool {
    if o, ok := other.(UInt8); ok {
        return self < o
    } else {
        return false
    }
}

func (self UInt8) ByteSize() int {
    return 1
}

func (self UInt8) SaveTo(b []byte) int {
    if cap(b) < 1 { panic("buf too small") }
    b[0] = byte(self)
    return 1
}

func LoadUInt8(bytes []byte) UInt8 {
    return UInt8(bytes[0])
}


// Int16

func (self Int16) Equals(other Key) bool {
    if o, ok := other.(Int16); ok {
        return self == o
    } else {
        return false
    }
}

func (self Int16) Less(other Key) bool {
    if o, ok := other.(Int16); ok {
        return self < o
    } else {
        return false
    }
}

func (self Int16) ByteSize() int {
    return 2
}

func (self Int16) SaveTo(b []byte) int {
    if cap(b) < 2 { panic("buf too small") }
    binary.LittleEndian.PutUint16(b, uint16(self))
    return 2
}

func LoadInt16(bytes []byte) Int16 {
    return Int16(binary.LittleEndian.Uint16(bytes))
}


// UInt16

func (self UInt16) Equals(other Key) bool {
    if o, ok := other.(UInt16); ok {
        return self == o
    } else {
        return false
    }
}

func (self UInt16) Less(other Key) bool {
    if o, ok := other.(UInt16); ok {
        return self < o
    } else {
        return false
    }
}

func (self UInt16) ByteSize() int {
    return 2
}

func (self UInt16) SaveTo(b []byte) int {
    if cap(b) < 2 { panic("buf too small") }
    binary.LittleEndian.PutUint16(b, uint16(self))
    return 2
}

func LoadUInt16(bytes []byte) UInt16 {
    return UInt16(binary.LittleEndian.Uint16(bytes))
}


// Int32

func (self Int32) Equals(other Key) bool {
    if o, ok := other.(Int32); ok {
        return self == o
    } else {
        return false
    }
}

func (self Int32) Less(other Key) bool {
    if o, ok := other.(Int32); ok {
        return self < o
    } else {
        return false
    }
}

func (self Int32) ByteSize() int {
    return 4
}

func (self Int32) SaveTo(b []byte) int {
    if cap(b) < 4 { panic("buf too small") }
    binary.LittleEndian.PutUint32(b, uint32(self))
    return 4
}

func LoadInt32(bytes []byte) Int32 {
    return Int32(binary.LittleEndian.Uint32(bytes))
}


// UInt32

func (self UInt32) Equals(other Key) bool {
    if o, ok := other.(UInt32); ok {
        return self == o
    } else {
        return false
    }
}

func (self UInt32) Less(other Key) bool {
    if o, ok := other.(UInt32); ok {
        return self < o
    } else {
        return false
    }
}

func (self UInt32) ByteSize() int {
    return 4
}

func (self UInt32) SaveTo(b []byte) int {
    if cap(b) < 4 { panic("buf too small") }
    binary.LittleEndian.PutUint32(b, uint32(self))
    return 4
}

func LoadUInt32(bytes []byte) UInt32 {
    return UInt32(binary.LittleEndian.Uint32(bytes))
}


// Int64

func (self Int64) Equals(other Key) bool {
    if o, ok := other.(Int64); ok {
        return self == o
    } else {
        return false
    }
}

func (self Int64) Less(other Key) bool {
    if o, ok := other.(Int64); ok {
        return self < o
    } else {
        return false
    }
}

func (self Int64) ByteSize() int {
    return 8
}

func (self Int64) SaveTo(b []byte) int {
    if cap(b) < 8 { panic("buf too small") }
    binary.LittleEndian.PutUint64(b, uint64(self))
    return 8
}

func LoadInt64(bytes []byte) Int64 {
    return Int64(binary.LittleEndian.Uint64(bytes))
}


// UInt64

func (self UInt64) Equals(other Key) bool {
    if o, ok := other.(UInt64); ok {
        return self == o
    } else {
        return false
    }
}

func (self UInt64) Less(other Key) bool {
    if o, ok := other.(UInt64); ok {
        return self < o
    } else {
        return false
    }
}

func (self UInt64) ByteSize() int {
    return 8
}

func (self UInt64) SaveTo(b []byte) int {
    if cap(b) < 8 { panic("buf too small") }
    binary.LittleEndian.PutUint64(b, uint64(self))
    return 8
}

func LoadUInt64(bytes []byte) UInt64 {
    return UInt64(binary.LittleEndian.Uint64(bytes))
}


// Int

func (self Int) Equals(other Key) bool {
    if o, ok := other.(Int); ok {
        return self == o
    } else {
        return false
    }
}

func (self Int) Less(other Key) bool {
    if o, ok := other.(Int); ok {
        return self < o
    } else {
        return false
    }
}

func (self Int) ByteSize() int {
    return 8
}

func (self Int) SaveTo(b []byte) int {
    if cap(b) < 8 { panic("buf too small") }
    binary.LittleEndian.PutUint64(b, uint64(self))
    return 8
}

func LoadInt(bytes []byte) Int {
    return Int(binary.LittleEndian.Uint64(bytes))
}

// UInt

func (self UInt) Equals(other Key) bool {
    if o, ok := other.(UInt); ok {
        return self == o
    } else {
        return false
    }
}

func (self UInt) Less(other Key) bool {
    if o, ok := other.(UInt); ok {
        return self < o
    } else {
        return false
    }
}

func (self UInt) ByteSize() int {
    return 8
}

func (self UInt) SaveTo(b []byte) int {
    if cap(b) < 8 { panic("buf too small") }
    binary.LittleEndian.PutUint64(b, uint64(self))
    return 8
}

func LoadUInt(bytes []byte) UInt {
    return UInt(binary.LittleEndian.Uint64(bytes))
}
