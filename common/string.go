package common

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// Fmt shorthand, XXX DEPRECATED
var Fmt = fmt.Sprintf

// RightPadString adds spaces to the right of a string to make it length totalLength
func RightPadString(s string, totalLength int) string {
	remaining := totalLength - len(s)
	if remaining > 0 {
		s = s + strings.Repeat(" ", remaining)
	}
	return s
}

// LeftPadString adds spaces to the left of a string to make it length totalLength
func LeftPadString(s string, totalLength int) string {
	remaining := totalLength - len(s)
	if remaining > 0 {
		s = strings.Repeat(" ", remaining) + s
	}
	return s
}

// IsHex returns true for non-empty hex-string prefixed with "0x"
func IsHex(s string) bool {
	if len(s) > 2 && s[:2] == "0x" {
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
