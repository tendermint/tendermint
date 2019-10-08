package query

// Empty query matches any set of events.
type Empty struct {
}

// Matches always returns true.
func (Empty) Matches(events map[string][]string) bool {
	return true
}

func (Empty) String() string {
	return "empty"
}
