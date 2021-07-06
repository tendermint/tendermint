package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultValue(t *testing.T) {
	t.Parallel()
	v := NewBool(false)
	assert.False(t, v.IsSet())

	v = NewBool(true)
	assert.True(t, v.IsSet())
}

func TestSetUnSet(t *testing.T) {
	t.Parallel()
	v := NewBool(false)

	v.Set()
	assert.True(t, v.IsSet())

	v.UnSet()
	assert.False(t, v.IsSet())
}
