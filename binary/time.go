package binary

import (
	"io"
	"time"
)

// Time

func WriteTime(t time.Time, w io.Writer, n *int64, err *error) {
	WriteInt64(t.UnixNano(), w, n, err)
}

func ReadTime(r io.Reader, n *int64, err *error) time.Time {
	t := ReadInt64(r, n, err)
	return time.Unix(0, t)
}
