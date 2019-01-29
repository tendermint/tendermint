package pubsub

// TagMap is used to associate tags to a message.
// They can be queried by subscribers to choose messages they will received.
type TagMap interface {
	// Get returns the value for a key, or nil if no value is present.
	// The ok result indicates whether value was found in the tags.
	Get(key string) (value string, ok bool)
	// Len returns the number of tags.
	Len() int
}

type tagMap map[string]string

var _ TagMap = (*tagMap)(nil)

// NewTagMap constructs a new immutable tag set from a map.
func NewTagMap(data map[string]string) TagMap {
	return tagMap(data)
}

// Get returns the value for a key, or nil if no value is present.
// The ok result indicates whether value was found in the tags.
func (ts tagMap) Get(key string) (value string, ok bool) {
	value, ok = ts[key]
	return
}

// Len returns the number of tags.
func (ts tagMap) Len() int {
	return len(ts)
}
