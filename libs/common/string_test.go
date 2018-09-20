package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringInSlice(t *testing.T) {
	assert.True(t, StringInSlice("a", []string{"a", "b", "c"}))
	assert.False(t, StringInSlice("d", []string{"a", "b", "c"}))
	assert.True(t, StringInSlice("", []string{""}))
	assert.False(t, StringInSlice("", []string{}))
}

func TestIsASCIIText(t *testing.T) {
	notASCIIText := []string{
		"", "\xC2", "\xC2\xA2", "\xFF", "\x80", "\xF0", "\n", "\t",
	}
	for _, v := range notASCIIText {
		assert.False(t, IsASCIIText(v), "%q is not ascii-text", v)
	}
	asciiText := []string{
		" ", ".", "x", "$", "_", "abcdefg;", "-", "0x00", "0", "123",
	}
	for _, v := range asciiText {
		assert.True(t, IsASCIIText(v), "%q is ascii-text", v)
	}
}

func TestASCIITrim(t *testing.T) {
	assert.Equal(t, ASCIITrim(" "), "")
	assert.Equal(t, ASCIITrim(" a"), "a")
	assert.Equal(t, ASCIITrim("a "), "a")
	assert.Equal(t, ASCIITrim(" a "), "a")
	assert.Panics(t, func() { ASCIITrim("\xC2\xA2") })
}
