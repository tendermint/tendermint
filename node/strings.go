package node

import "strings"

//move from string.go. filter out empty string too.
func SplitAndTrim(s, sep, cutset string) []string {
	if s == "" {
		return []string{}
	}

	spl := strings.Split(s, sep)
	nonEmptyString := make([]string, 0, len(spl))
	for i := 0; i < len(spl); i++ {
		element := strings.Trim(spl[i], cutset)
		if element != "" {
			nonEmptyString = append(nonEmptyString, element)
		}
	}
	return nonEmptyString
}
