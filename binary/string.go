package binary

import "io"

type String string

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
	return len(self) + 4
}

func (self String) WriteTo(w io.Writer) (n int64, err error) {
	var n_ int
	_, err = UInt32(len(self)).WriteTo(w)
	if err != nil {
		return n, err
	}
	n_, err = w.Write([]byte(self))
	return int64(n_ + 4), err
}

func ReadStringSafe(r io.Reader) (String, error) {
	length, err := ReadUInt32Safe(r)
	if err != nil {
		return "", err
	}
	bytes := make([]byte, int(length))
	_, err = io.ReadFull(r, bytes)
	if err != nil {
		return "", err
	}
	return String(bytes), nil
}

func ReadString(r io.Reader) String {
	str, err := ReadStringSafe(r)
	if r != nil {
		panic(err)
	}
	return str
}
