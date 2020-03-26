package math

import (
	"errors"
	"math"
)

var ErrOverflowInt = errors.New("integer overflow")

// SafeAddInt32 adds two int32 integers
// If there is an overflow 0, and ErrOverflowInt
func SafeAddInt32(a, b int32) (int32, error) {
	if b > 0 && (a > math.MaxInt32-b) {
		return 0, ErrOverflowInt
	} else if b < 0 && (a < math.MinInt32-b) {
		return 0, ErrOverflowInt
	}
	return a + b, nil
}

// SafeSubInt32 subtracts two int32 integers
// If there is an overflow 0, and ErrOverflowInt
func SafeSubInt32(a, b int32) (int32, error) {
	if b > 0 && (a < math.MinInt32+b) {
		return 0, ErrOverflowInt
	} else if b < 0 && (a > math.MaxInt32+b) {
		return 0, ErrOverflowInt
	}
	return a - b, nil
}
