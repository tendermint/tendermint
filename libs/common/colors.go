package common

import (
	"fmt"
	"strings"
)

const (
	ANSIReset      = "\x1b[0m"
	ANSIBright     = "\x1b[1m"
	ANSIDim        = "\x1b[2m"
	ANSIUnderscore = "\x1b[4m"
	ANSIBlink      = "\x1b[5m"
	ANSIReverse    = "\x1b[7m"
	ANSIHidden     = "\x1b[8m"

	ANSIFgBlack   = "\x1b[30m"
	ANSIFgRed     = "\x1b[31m"
	ANSIFgGreen   = "\x1b[32m"
	ANSIFgYellow  = "\x1b[33m"
	ANSIFgBlue    = "\x1b[34m"
	ANSIFgMagenta = "\x1b[35m"
	ANSIFgCyan    = "\x1b[36m"
	ANSIFgWhite   = "\x1b[37m"

	ANSIBgBlack   = "\x1b[40m"
	ANSIBgRed     = "\x1b[41m"
	ANSIBgGreen   = "\x1b[42m"
	ANSIBgYellow  = "\x1b[43m"
	ANSIBgBlue    = "\x1b[44m"
	ANSIBgMagenta = "\x1b[45m"
	ANSIBgCyan    = "\x1b[46m"
	ANSIBgWhite   = "\x1b[47m"
)

// color the string s with color 'color'
// unless s is already colored
func treat(s string, color string) string {
	if len(s) > 2 && s[:2] == "\x1b[" {
		return s
	}
	return color + s + ANSIReset
}

func treatAll(color string, args ...interface{}) string {
	var parts []string
	for _, arg := range args {
		parts = append(parts, treat(fmt.Sprintf("%v", arg), color))
	}
	return strings.Join(parts, "")
}

func Black(args ...interface{}) string {
	return treatAll(ANSIFgBlack, args...)
}

func Red(args ...interface{}) string {
	return treatAll(ANSIFgRed, args...)
}

func Green(args ...interface{}) string {
	return treatAll(ANSIFgGreen, args...)
}

func Yellow(args ...interface{}) string {
	return treatAll(ANSIFgYellow, args...)
}

func Blue(args ...interface{}) string {
	return treatAll(ANSIFgBlue, args...)
}

func Magenta(args ...interface{}) string {
	return treatAll(ANSIFgMagenta, args...)
}

func Cyan(args ...interface{}) string {
	return treatAll(ANSIFgCyan, args...)
}

func White(args ...interface{}) string {
	return treatAll(ANSIFgWhite, args...)
}

func ColoredBytes(data []byte, textColor, bytesColor func(...interface{}) string) string {
	s := ""
	for _, b := range data {
		if 0x21 <= b && b < 0x7F {
			s += textColor(string(b))
		} else {
			s += bytesColor(fmt.Sprintf("%02X", b))
		}
	}
	return s
}
