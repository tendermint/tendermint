package common

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// Like fmt.Sprintf, but skips formatting if args are empty.
var Fmt = func(format string, a ...interface{}) string {
	if len(a) == 0 {
		return format
	}
	return fmt.Sprintf(format, a...)
}

// IsHex returns true for non-empty hex-string prefixed with "0x"
func IsHex(s string) bool {
	if len(s) > 2 && strings.EqualFold(s[:2], "0x") {
		_, err := hex.DecodeString(s[2:])
		return err == nil
	}
	return false
}

// StripHex returns hex string without leading "0x"
func StripHex(s string) string {
	if IsHex(s) {
		return s[2:]
	}
	return s
}

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
