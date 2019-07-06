package query

// Empty query matches any set of tags.
type Empty struct {
}

// Matches always returns true.
func (Empty) Matches(tags map[string][]string) bool {
	return true
}

func (Empty) String() string {
	return "empty"
}
