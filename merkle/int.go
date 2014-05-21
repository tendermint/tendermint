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

func (self Int8) Bytes() []byte {
    return []byte{byte(self)}
}


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

func (self UInt8) Bytes() []byte {
    return []byte{byte(self)}
}


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

func (self Int16) Bytes() []byte {
    b := [2]byte{}
    binary.LittleEndian.PutUint16(b[:], uint16(self))
    return b[:]
}


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

func (self UInt16) Bytes() []byte {
    b := [2]byte{}
    binary.LittleEndian.PutUint16(b[:], uint16(self))
    return b[:]
}


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

func (self Int32) Bytes() []byte {
    b := [4]byte{}
    binary.LittleEndian.PutUint32(b[:], uint32(self))
    return b[:]
}


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

func (self UInt32) Bytes() []byte {
    b := [4]byte{}
    binary.LittleEndian.PutUint32(b[:], uint32(self))
    return b[:]
}


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

func (self Int64) Bytes() []byte {
    b := [8]byte{}
    binary.LittleEndian.PutUint64(b[:], uint64(self))
    return b[:]
}


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

func (self UInt64) Bytes() []byte {
    b := [8]byte{}
    binary.LittleEndian.PutUint64(b[:], uint64(self))
    return b[:]
}


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

func (self Int) Bytes() []byte {
    b := [8]byte{}
    binary.LittleEndian.PutUint64(b[:], uint64(self))
    return b[:]
}


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

func (self UInt) Bytes() []byte {
    b := [8]byte{}
    binary.LittleEndian.PutUint64(b[:], uint64(self))
    return b[:]
}
