package merkle

import "bytes"

type String string
type ByteSlice []byte

// String

func (self String) Equals(other Key) bool {
    if o, ok := other.(String); ok {
        return self == o
    } else {
        return false
    }
}

func (self String) Less(other Key) bool {
    if o, ok := other.(String); ok {
        return self < o
    } else {
        return false
    }
}

func (self String) ByteSize() int {
    return len(self)+4
}

func (self String) SaveTo(buf []byte) int {
    if len(buf) < self.ByteSize() { panic("buf too small") }
    UInt32(len(self)).SaveTo(buf)
    copy(buf[4:], []byte(self))
    return len(self)+4
}

func LoadString(bytes []byte) String {
    length := LoadUInt32(bytes)
    return String(bytes[4:4+length])
}


// ByteSlice

func (self ByteSlice) Equals(other Key) bool {
    if o, ok := other.(ByteSlice); ok {
        return bytes.Equal(self, o)
    } else {
        return false
    }
}

func (self ByteSlice) Less(other Key) bool {
    if o, ok := other.(ByteSlice); ok {
        return bytes.Compare(self, o) < 0 // -1 if a < b
    } else {
        return false
    }
}

func (self ByteSlice) ByteSize() int {
    return len(self)+4
}

func (self ByteSlice) SaveTo(buf []byte) int {
    if len(buf) < self.ByteSize() { panic("buf too small") }
    UInt32(len(self)).SaveTo(buf)
    copy(buf[4:], self)
    return len(self)+4
}

func LoadByteSlice(bytes []byte) ByteSlice {
    length := LoadUInt32(bytes)
    return ByteSlice(bytes[4:4+length])
}
