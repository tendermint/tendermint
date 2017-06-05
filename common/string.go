package common

import (
	"encoding/hex"
	"fmt"
	"strings"
)

var Fmt = fmt.Sprintf

func RightPadString(s string, totalLength int) string {
	remaining := totalLength - len(s)
	if remaining > 0 {
		s = s + strings.Repeat(" ", remaining)
	}
	return s
}

func LeftPadString(s string, totalLength int) string {
	remaining := totalLength - len(s)
	if remaining > 0 {
		s = strings.Repeat(" ", remaining) + s
	}
	return s
}

// Returns true for non-empty hex-string prefixed with "0x"
func IsHex(s string) bool {
	if len(s) > 2 && s[:2] == "0x" {
		_, err := hex.DecodeString(s[2:])
		if err != nil {
			return false
		}
		return true
	}
	return false
}

func StripHex(s string) string {
	if IsHex(s) {
		return s[2:]
	}
	return s
}
