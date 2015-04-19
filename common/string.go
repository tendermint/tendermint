package common

import (
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
