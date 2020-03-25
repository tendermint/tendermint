package math

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddInt32Overflow(t *testing.T) {
	testCases := []struct {
		value    int32
		expError bool
	}{
		{-2, true},
		{-1, true},
		{1, false},
		{2, false},
	}

	for _, tc := range testCases {
		_, err := SafeAddInt32(tc.value, math.MaxInt32)
		assert.Equal(t, tc.expError, err == nil)
		_, err = SafeAddInt32(math.MaxInt32, tc.value)
		assert.Equal(t, tc.expError, err == nil)
		_, err = SafeAddInt32(tc.value, math.MinInt32)
		assert.Equal(t, tc.expError, err != nil)
		_, err = SafeAddInt32(math.MinInt32, tc.value)
		assert.Equal(t, tc.expError, err != nil)
	}
}
func TestSubInt32Overflow(t *testing.T) {
	testCases := []struct {
		value    int32
		expError bool
	}{
		{-2, false},
		{1, true},
		{2, true},
		{5, true},
	}

	for _, tc := range testCases {
		_, err := SafeSubInt32(tc.value, math.MaxInt32)
		assert.Equal(t, tc.expError, err == nil)
		_, err = SafeSubInt32(math.MaxInt32, tc.value)
		assert.Equal(t, tc.expError, err == nil)
		_, err = SafeSubInt32(tc.value, math.MinInt32)
		assert.Equal(t, tc.expError, err != nil)
		_, err = SafeSubInt32(math.MinInt32, tc.value)
		assert.Equal(t, tc.expError, err != nil)
	}
}
