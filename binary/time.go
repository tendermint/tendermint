package binary

import (
	"io"
	"time"
)

type Time struct {
	time.Time
}

func (self Time) Equals(other Binary) bool {
	if o, ok := other.(Time); ok {
		return self.Equal(o.Time)
	} else {
		return false
	}
}

func (self Time) Less(other Binary) bool {
	if o, ok := other.(Time); ok {
		return self.Before(o.Time)
	} else {
		panic("Cannot compare unequal types")
	}
}

func (self Time) ByteSize() int {
	return 8
}

func (self Time) WriteTo(w io.Writer) (int64, error) {
	return Int64(self.Unix()).WriteTo(w)
}

func ReadTime(r io.Reader) Time {
	return Time{time.Unix(int64(ReadInt64(r)), 0)}
}
