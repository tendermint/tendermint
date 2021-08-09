package math

import (
	"errors"
	"math"
)

var ErrOverflowInt32 = errors.New("int32 overflow")
var ErrOverflowUint8 = errors.New("uint8 overflow")
var ErrOverflowInt8 = errors.New("int8 overflow")

// SafeAdd adds two int64 numbers. If there is an overflow,
// the function will return -1, true
func SafeAdd(a, b int64) (int64, bool) {
	if b > 0 && a > math.MaxInt64-b {
		return -1, true
	} else if b < 0 && a < math.MinInt64-b {
		return -1, true
	}
	return a + b, false
}

// SafeSub subtracts two int64 numbers. If there is an overflow,
// the function will return -1, true
func SafeSub(a, b int64) (int64, bool) {
	if b > 0 && a < math.MinInt64+b {
		return -1, true
	} else if b < 0 && a > math.MaxInt64+b {
		return -1, true
	}
	return a - b, false
}

// SafeAddClip performs SafeAdd, however if there is an overflow,
// it will return the maxInt64
func SafeAddClip(a, b int64) int64 {
	c, overflow := SafeAdd(a, b)
	if overflow {
		if b < 0 {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return c
}

// SafeSubClip performs SafeSub but will clip the result to either
// the min or max int64 number
func SafeSubClip(a, b int64) int64 {
	c, overflow := SafeSub(a, b)
	if overflow {
		if b > 0 {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return c
}

// SafeMul multiplies two int64 numbers. It returns
// true if the resultant overflows.
func SafeMul(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, false
	}

	absOfB := b
	if b < 0 {
		absOfB = -b
	}

	absOfA := a
	if a < 0 {
		absOfA = -a
	}

	if absOfA > math.MaxInt64/absOfB {
		return 0, true
	}

	return a * b, false
}

// SafeAddInt32 adds two int32 integers
// If there is an overflow this will panic
func SafeAddInt32(a, b int32) int32 {
	if b > 0 && (a > math.MaxInt32-b) {
		panic(ErrOverflowInt32)
	} else if b < 0 && (a < math.MinInt32-b) {
		panic(ErrOverflowInt32)
	}
	return a + b
}

// SafeSubInt32 subtracts two int32 integers
// If there is an overflow this will panic
func SafeSubInt32(a, b int32) int32 {
	if b > 0 && (a < math.MinInt32+b) {
		panic(ErrOverflowInt32)
	} else if b < 0 && (a > math.MaxInt32+b) {
		panic(ErrOverflowInt32)
	}
	return a - b
}

// SafeConvertInt32 takes a int and checks if it overflows
// If there is an overflow this will panic
func SafeConvertInt32(a int64) int32 {
	if a > math.MaxInt32 {
		panic(ErrOverflowInt32)
	} else if a < math.MinInt32 {
		panic(ErrOverflowInt32)
	}
	return int32(a)
}

// SafeConvertUint8 takes an int64 and checks if it overflows
// If there is an overflow it returns an error
func SafeConvertUint8(a int64) (uint8, error) {
	if a > math.MaxUint8 {
		return 0, ErrOverflowUint8
	} else if a < 0 {
		return 0, ErrOverflowUint8
	}
	return uint8(a), nil
}

// SafeConvertInt8 takes an int64 and checks if it overflows
// If there is an overflow it returns an error
func SafeConvertInt8(a int64) (int8, error) {
	if a > math.MaxInt8 {
		return 0, ErrOverflowInt8
	} else if a < math.MinInt8 {
		return 0, ErrOverflowInt8
	}
	return int8(a), nil
}
