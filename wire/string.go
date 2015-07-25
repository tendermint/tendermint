package wire

import (
	"io"

	. "github.com/tendermint/tendermint/common"
)

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
	if length < 0 {
		*err = ErrBinaryReadSizeUnderflow
		return ""
	}
	if MaxBinaryReadSize < MaxInt64(int64(length), *n+int64(length)) {
		*err = ErrBinaryReadSizeOverflow
		return ""
	}

	buf := make([]byte, length)
	ReadFull(buf, r, n, err)
	return string(buf)
}
