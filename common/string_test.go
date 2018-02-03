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

func TestIsHex(t *testing.T) {
	notHex := []string{
		"", "   ", "a", "x", "0", "0x", "0X", "0x ", "0X ", "0X a",
		"0xf ", "0x f", "0xp", "0x-",
		"0xf", "0XBED", "0xF", "0xbed", // Odd lengths
	}
	for _, v := range notHex {
		assert.False(t, IsHex(v), "%q is not hex", v)
	}
	hex := []string{
		"0x00", "0x0a", "0x0F", "0xFFFFFF", "0Xdeadbeef", "0x0BED",
		"0X12", "0X0A",
	}
	for _, v := range hex {
		assert.True(t, IsHex(v), "%q is hex", v)
	}
}
