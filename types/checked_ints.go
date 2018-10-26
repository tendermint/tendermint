package types

import (
	"errors"
	"math"
)

var ErrOverflowInt = errors.New("integer overflow")

type CheckedInt32 int32
type CheckedUint32 uint32

type CheckedInt64 int64
type CheckedUint64 int64

func (i32 CheckedInt32) CheckedAdd(otherI32 CheckedInt32) (CheckedInt32, error) {
	if otherI32 > 0 && (i32 > math.MaxInt32-otherI32) {
		return 0, ErrOverflowInt
	} else if otherI32 < 0 && (i32 < math.MinInt32-otherI32) {
		return 0, ErrOverflowInt
	}
	return i32 + otherI32, nil
}

func (i32 CheckedInt32) CheckedSub(otherI32 CheckedInt32) (CheckedInt32, error) {
	if otherI32 > 0 && (i32 < math.MinInt32+otherI32) {
		return 0, ErrOverflowInt
	} else if otherI32 < 0 && (i32 > math.MaxInt32+otherI32) {
		return 0, ErrOverflowInt
	}
	return i32 - otherI32, nil

}
