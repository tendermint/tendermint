package strings

import (
	"fmt"
	"strings"
)

// SplitAndTrimEmpty slices s into all subslices separated by sep and returns a
// slice of the string s with all leading and trailing Unicode code points
// contained in cutset removed. If sep is empty, SplitAndTrim splits after each
// UTF-8 sequence. First part is equivalent to strings.SplitN with a count of
// -1.  also filter out empty strings, only return non-empty strings.
func SplitAndTrimEmpty(s, sep, cutset string) []string {
	if s == "" {
		return []string{}
	}

	spl := strings.Split(s, sep)
	nonEmptyStrings := make([]string, 0, len(spl))

	for i := 0; i < len(spl); i++ {
		element := strings.Trim(spl[i], cutset)
		if element != "" {
			nonEmptyStrings = append(nonEmptyStrings, element)
		}
	}

	return nonEmptyStrings
}

// ASCIITrim removes spaces from an a ASCII string, erroring if the
// sequence is not an ASCII string.
func ASCIITrim(s string) (string, error) {
	if len(s) == 0 {
		return "", nil
	}
	r := make([]byte, 0, len(s))
	for _, b := range []byte(s) {
		switch {
		case b == 32:
			continue // skip space
		case 32 < b && b <= 126:
			r = append(r, b)
		default:
			return "", fmt.Errorf("non-ASCII (non-tab) char 0x%X", b)
		}
	}
	return string(r), nil
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
