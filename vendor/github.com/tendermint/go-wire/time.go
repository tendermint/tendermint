package wire

import (
	"io"
	"time"

	cmn "github.com/tendermint/tmlibs/common"
)

// WriteTime writes the number of nanoseconds, with millisecond resolution,
// since January 1, 1970 UTC, to the Writer as an Int64.
// Milliseconds are used to ease compatibility with Javascript,
// which does not support finer resolution.
// NOTE: panics if the given time is less than January 1, 1970 UTC
func WriteTime(t time.Time, w io.Writer, n *int, err *error) {
	nanosecs := t.UnixNano()
	millisecs := nanosecs / 1000000
	if nanosecs < 0 {
		cmn.PanicSanity("can't encode times below 1970")
	} else {
		WriteInt64(millisecs*1000000, w, n, err)
	}
}

// ReadTime reads an Int64 from the Reader, interprets it as
// the number of nanoseconds since January 1, 1970 UTC, and
// returns the corresponding time. If the Int64 read is less than zero,
// or not a multiple of a million, it sets the error and returns the default time.
func ReadTime(r io.Reader, n *int, err *error) time.Time {
	t := ReadInt64(r, n, err)
	if t < 0 {
		*err = ErrBinaryReadInvalidTimeNegative
		return time.Time{}
	}
	if t%1000000 != 0 {
		*err = ErrBinaryReadInvalidTimeSubMillisecond
		return time.Time{}
	}
	return time.Unix(0, t)
}
