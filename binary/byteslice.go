package binary

import "io"
import "bytes"

type ByteSlice []byte

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
	return len(self) + 4
}

func (self ByteSlice) WriteTo(w io.Writer) (n int64, err error) {
	var n_ int
	_, err = UInt32(len(self)).WriteTo(w)
	if err != nil {
		return n, err
	}
	n_, err = w.Write([]byte(self))
	return int64(n_ + 4), err
}

func ReadByteSliceSafe(r io.Reader) (ByteSlice, error) {
	length, err := ReadUInt32Safe(r)
	if err != nil {
		return nil, err
	}
	bytes := make([]byte, int(length))
	_, err = io.ReadFull(r, bytes)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func ReadByteSlice(r io.Reader) ByteSlice {
	bytes, err := ReadByteSliceSafe(r)
	if r != nil {
		panic(err)
	}
	return bytes
}
