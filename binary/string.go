package binary

import "io"

// String

func WriteString(s string, w io.Writer, n *int64, err *error) {
	WriteUInt32(uint32(len(s)), w, n, err)
	WriteTo([]byte(s), w, n, err)
}

func ReadString(r io.Reader, n *int64, err *error) string {
	length := ReadUInt32(r, n, err)
	if *err != nil {
		return ""
	}
	buf := make([]byte, int(length))
	ReadFull(buf, r, n, err)
	return string(buf)
}
