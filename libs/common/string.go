package common

import (
	"fmt"
	"strings"
)

// StringInSlice returns true if a is found the list.
func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// SplitAndTrim slices s into all subslices separated by sep and returns a
// slice of the string s with all leading and trailing Unicode code points
// contained in cutset removed. If sep is empty, SplitAndTrim splits after each
// UTF-8 sequence. First part is equivalent to strings.SplitN with a count of
// -1.
func SplitAndTrim(s, sep, cutset string) []string {
	if s == "" {
		return []string{}
	}

	spl := strings.Split(s, sep)
	for i := 0; i < len(spl); i++ {
		spl[i] = strings.Trim(spl[i], cutset)
	}
	return spl
}

// Returns true if s is a non-empty printable non-tab ascii character.
func IsASCIIText(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, b := range []byte(s) {
		if 32 <= b && b <= 126 {
			// good
		} else {
			return false
		}
	}
	return true
}

// NOTE: Assumes that s is ASCII as per IsASCIIText(), otherwise panics.
func ASCIITrim(s string) string {
	r := make([]byte, 0, len(s))
	for _, b := range []byte(s) {
		if b == 32 {
			continue // skip space
		} else if 32 < b && b <= 126 {
			r = append(r, b)
		} else {
			panic(fmt.Sprintf("non-ASCII (non-tab) char 0x%X", b))
		}
	}
	return string(r)
}

// StringSliceEqual checks if string slices a and b are equal
func StringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
