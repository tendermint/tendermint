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

func ReadTimeSafe(r io.Reader) (Time, int64, error) {
	t, n, err := ReadInt64Safe(r)
	if err != nil {
		return Time{}, n, err
	}
	return Time{time.Unix(int64(t), 0)}, n, nil
}

func ReadTimeN(r io.Reader) (Time, int64) {
	t, n, err := ReadTimeSafe(r)
	if err != nil {
		panic(err)
	}
	return t, n
}

func ReadTime(r io.Reader) Time {
	t, _, err := ReadTimeSafe(r)
	if err != nil {
		panic(err)
	}
	return t
}
