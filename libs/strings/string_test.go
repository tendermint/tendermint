package strings

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitAndTrimEmpty(t *testing.T) {
	testCases := []struct {
		s        string
		sep      string
		cutset   string
		expected []string
	}{
		{"a,b,c", ",", " ", []string{"a", "b", "c"}},
		{" a , b , c ", ",", " ", []string{"a", "b", "c"}},
		{" a, b, c ", ",", " ", []string{"a", "b", "c"}},
		{" a, ", ",", " ", []string{"a"}},
		{"   ", ",", " ", []string{}},
	}

	for _, tc := range testCases {
		require.Equal(t, tc.expected, SplitAndTrimEmpty(tc.s, tc.sep, tc.cutset), "%s", tc.s)
	}
}

func TestStringInSlice(t *testing.T) {
	require.True(t, StringInSlice("a", []string{"a", "b", "c"}))
	require.False(t, StringInSlice("d", []string{"a", "b", "c"}))
	require.True(t, StringInSlice("", []string{""}))
	require.False(t, StringInSlice("", []string{}))
}

func TestIsASCIIText(t *testing.T) {
	notASCIIText := []string{
		"", "\xC2", "\xC2\xA2", "\xFF", "\x80", "\xF0", "\n", "\t",
	}
	for _, v := range notASCIIText {
		require.False(t, IsASCIIText(v), "%q is not ascii-text", v)
	}
	asciiText := []string{
		" ", ".", "x", "$", "_", "abcdefg;", "-", "0x00", "0", "123",
	}
	for _, v := range asciiText {
		require.True(t, IsASCIIText(v), "%q is ascii-text", v)
	}
}

func TestASCIITrim(t *testing.T) {
	require.Equal(t, ASCIITrim(" "), "")
	require.Equal(t, ASCIITrim(" a"), "a")
	require.Equal(t, ASCIITrim("a "), "a")
	require.Equal(t, ASCIITrim(" a "), "a")
	require.Panics(t, func() { ASCIITrim("\xC2\xA2") })
}

func TestStringSliceEqual(t *testing.T) {
	tests := []struct {
		a    []string
		b    []string
		want bool
	}{
		{[]string{"hello", "world"}, []string{"hello", "world"}, true},
		{[]string{"test"}, []string{"test"}, true},
		{[]string{"test1"}, []string{"test2"}, false},
		{[]string{"hello", "world."}, []string{"hello", "world!"}, false},
		{[]string{"only 1 word"}, []string{"two", "words!"}, false},
		{[]string{"two", "words!"}, []string{"only 1 word"}, false},
	}
	for i, tt := range tests {
		require.Equal(t, tt.want, StringSliceEqual(tt.a, tt.b),
			"StringSliceEqual failed on test %d", i)
	}
}
