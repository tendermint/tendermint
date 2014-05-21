package merkle

import "bytes"

type String string
type ByteSlice []byte

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

func (self String) Bytes() []byte {
    return []byte(self)
}

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

func (self ByteSlice) Bytes() []byte {
    return []byte(self)
}
