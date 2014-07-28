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

func (self ByteSlice) Reader() io.Reader {
	return bytes.NewReader([]byte(self))
}

func ReadByteSliceSafe(r io.Reader) (bytes ByteSlice, n int64, err error) {
	length, n_, err := ReadUInt32Safe(r)
	n += n_
	if err != nil {
		return nil, n, err
	}
	bytes = make([]byte, int(length))
	n__, err := io.ReadFull(r, bytes)
	n += int64(n__)
	if err != nil {
		return nil, n, err
	}
	return bytes, n, nil
}

func ReadByteSliceN(r io.Reader) (bytes ByteSlice, n int64) {
	bytes, n, err := ReadByteSliceSafe(r)
	if err != nil {
		panic(err)
	}
	return bytes, n
}

func ReadByteSlice(r io.Reader) (bytes ByteSlice) {
	bytes, _, err := ReadByteSliceSafe(r)
	if err != nil {
		panic(err)
	}
	return bytes
}
