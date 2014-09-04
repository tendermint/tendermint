package binary

import "io"

// String

func WriteString(w io.Writer, s string, n *int64, err *error) {
	WriteUInt32(w, uint32(len(s)), n, err)
	WriteTo(w, []byte(s), n, err)
}

func ReadString(r io.Reader, n *int64, err *error) string {
	length := ReadUInt32(r, n, err)
	if *err != nil {
		return ""
	}
	buf := make([]byte, int(length))
	ReadFull(r, buf, n, err)
	return string(buf)
}
