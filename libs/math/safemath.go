package math

import (
	"errors"
	"math"
)

var ErrOverflowInt = errors.New("integer overflow")

func SafeAddInt32(a, b int32) (int32, error) {
	if b > 0 && (a > math.MaxInt32-b) {
		return 0, ErrOverflowInt
	} else if b < 0 && (a < math.MinInt32-b) {
		return 0, ErrOverflowInt
	}
	return a + b, nil
}

func SafeSubInt32(a, b int32) (int32, error) {
	if b > 0 && (a < math.MinInt32+b) {
		return 0, ErrOverflowInt
	} else if b < 0 && (a > math.MaxInt32+b) {
		return 0, ErrOverflowInt
	}
	return a - b, nil
}
