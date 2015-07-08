package binary

import "io"

// String

func WriteString(s string, w io.Writer, n *int64, err *error) {
	WriteVarint(len(s), w, n, err)
	WriteTo([]byte(s), w, n, err)
}

func ReadString(r io.Reader, n *int64, err *error) string {
	length := ReadVarint(r, n, err)
	if *err != nil {
		return ""
	}
	if MaxBinaryReadSize < *n+int64(length) {
		*err = ErrMaxBinaryReadSizeReached
		return ""
	}

	buf := make([]byte, length)
	ReadFull(buf, r, n, err)
	return string(buf)
}
