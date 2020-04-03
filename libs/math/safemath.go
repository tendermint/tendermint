package math

import (
	"errors"
	"math"
)

var ErrOverflowInt32 = errors.New("int32 overflow")

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

// SafeConvertUint32 takes a int and checks if it overflows
// If there is an overflow this will panic
func SafeConvertUint32(a int64) uint32 {
	if a > math.MaxUint32 {
		panic(ErrOverflowInt32)
	} else if a < 0 {
		panic(ErrOverflowInt32)
	}
	return uint32(a)
}
