package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntInSlice(t *testing.T) {
	assert.True(t, IntInSlice(1, []int{1, 2, 3}))
	assert.False(t, IntInSlice(4, []int{1, 2, 3}))
	assert.True(t, IntInSlice(0, []int{0}))
	assert.False(t, IntInSlice(0, []int{}))
}
