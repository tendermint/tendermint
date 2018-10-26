package types

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckedInt32_CheckedAdd(t *testing.T) {
	tcs := []struct {
		val1    int32
		val2    int32
		sum     int32
		wantErr bool
	}{
		0: {math.MaxInt32, 1, 0, true},
		1: {math.MinInt32, 1, math.MinInt32 + 1, false},
		2: {math.MaxInt32, math.MaxInt32, 0, true},
		3: {0, math.MaxInt32, math.MaxInt32, false},
		4: {0, 1, 1, false},
		5: {1, 1, 2, false},
	}
	for i, tc := range tcs {
		v1 := CheckedInt32(tc.val1)
		v2 := CheckedInt32(tc.val2)
		sum, err := v1.CheckedAdd(v2)
		if tc.wantErr {
			assert.Error(t, err, "Should fail: %v", i)
			assert.Zero(t, sum, "Got invalid sum for case %v", i)
		} else {
			assert.EqualValues(t, tc.sum, sum)
		}
	}
}

func TestCheckedInt32_CheckedSub(t *testing.T) {
	tcs := []struct {
		val1    int32
		val2    int32
		sum     int32
		wantErr bool
	}{
		0: {math.MaxInt32, math.MaxInt32, 0, false},
		1: {math.MinInt32, 1, 0, true},
		2: {math.MinInt32 + 1, 1, math.MinInt32, false},
		3: {1, 1, 0, false},
		4: {1, 2, -1, false},
	}
	for i, tc := range tcs {
		v1 := CheckedInt32(tc.val1)
		v2 := CheckedInt32(tc.val2)
		sum, err := v1.CheckedSub(v2)
		if tc.wantErr {
			assert.Error(t, err, "Should fail: %v", i)
			assert.Zero(t, sum, "Got invalid sum for case %v", i)
		} else {
			assert.EqualValues(t, tc.sum, sum, "failed: %v", i)
		}
	}
}
