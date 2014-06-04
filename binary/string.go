package binary

import "io"
import "bytes"

type String string
type ByteSlice []byte

// String

func (self String) Equals(other Binary) bool {
    return self == other
}

func (self String) Less(other Binary) bool {
    if o, ok := other.(String); ok {
        return self < o
    } else {
        panic("Cannot compare unequal types")
    }
}

func (self String) ByteSize() int {
    return len(self)+4
}

func (self String) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int
    _, err = UInt32(len(self)).WriteTo(w)
    if err != nil { return n, err }
    n_, err = w.Write([]byte(self))
    return int64(n_+4), err
}

// NOTE: keeps a reference to the original byte slice
func ReadString(bytes []byte, start int) (String, int) {
    length := int(ReadUInt32(bytes[start:]))
    return String(bytes[start+4:start+4+length]), start+4+length
}


// ByteSlice

func (self ByteSlice) Equals(other Binary) bool {
    if o, ok := other.(ByteSlice); ok {
        return bytes.Equal(self, o)
    } else {
        return false
    }
}

func (self ByteSlice) Less(other Binary) bool {
    if o, ok := other.(ByteSlice); ok {
        return bytes.Compare(self, o) < 0 // -1 if a < b
    } else {
        panic("Cannot compare unequal types")
    }
}

func (self ByteSlice) ByteSize() int {
    return len(self)+4
}

func (self ByteSlice) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int
    _, err = UInt32(len(self)).WriteTo(w)
    if err != nil { return n, err }
    n_, err = w.Write([]byte(self))
    return int64(n_+4), err
}

// NOTE: keeps a reference to the original byte slice
func ReadByteSlice(bytes []byte, start int) (ByteSlice, int) {
    length := int(ReadUInt32(bytes[start:]))
    return ByteSlice(bytes[start+4:start+4+length]), start+4+length
}
