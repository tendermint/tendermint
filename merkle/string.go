package merkle

import "bytes"

type String string
type ByteSlice []byte

func (self String) Equals(other Sortable) bool {
    if o, ok := other.(String); ok {
        return self == o
    } else {
        return false
    }
}

func (self String) Less(other Sortable) bool {
    if o, ok := other.(String); ok {
        return self < o
    } else {
        return false
    }
}

func (self String) Hash() int {
    bytes := []byte(self)
    hash := 0
    for i, c := range bytes {
        hash += (i+1)*int(c)
    }
    return hash
}

func (self ByteSlice) Equals(other Sortable) bool {
    if o, ok := other.(ByteSlice); ok {
        return bytes.Equal(self, o)
    } else {
        return false
    }
}

func (self ByteSlice) Less(other Sortable) bool {
    if o, ok := other.(ByteSlice); ok {
        return bytes.Compare(self, o) < 0 // -1 if a < b
    } else {
        return false
    }
}

func (self ByteSlice) Hash() int {
    hash := 0
    for i, c := range self {
        hash += (i+1)*int(c)
    }
    return hash
}


