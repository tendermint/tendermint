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

func ReadStringSafe(r io.Reader) (str String, n int64, err error) {
	length, n_, err := ReadUInt32Safe(r)
	n += n_
	if err != nil {
		return "", n, err
	}
	bytes := make([]byte, int(length))
	n__, err := io.ReadFull(r, bytes)
	n += int64(n__)
	if err != nil {
		return "", n, err
	}
	return String(bytes), n, nil
}

func ReadStringN(r io.Reader) (str String, n int64) {
	str, n, err := ReadStringSafe(r)
	if err != nil {
		panic(err)
	}
	return str, n
}

func ReadString(r io.Reader) (str String) {
	str, _, err := ReadStringSafe(r)
	if err != nil {
		panic(err)
	}
	return str
}
