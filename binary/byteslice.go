package binary

import (
	"io"
)

// ByteSlice

func WriteByteSlice(w io.Writer, bz []byte, n *int64, err *error) {
	WriteUInt32(w, uint32(len(bz)), n, err)
	WriteTo(w, bz, n, err)
}

func ReadByteSlice(r io.Reader, n *int64, err *error) []byte {
	length := ReadUInt32(r, n, err)
	if *err != nil {
		return nil
	}
	buf := make([]byte, int(length))
	ReadFull(r, buf, n, err)
	return buf
}
