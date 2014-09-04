package binary

import (
	"io"
	"time"
)

// Time

func WriteTime(w io.Writer, t time.Time, n *int64, err *error) {
	WriteInt64(w, t.Unix(), n, err)
}

func ReadTime(r io.Reader, n *int64, err *error) time.Time {
	t := ReadInt64(r, n, err)
	return time.Unix(t, 0)
}
