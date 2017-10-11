package client

// ABCIQueryOptions can be used to provide options for ABCIQuery call other
// than the DefaultABCIQueryOptions.
type ABCIQueryOptions struct {
	Height  uint64
	Trusted bool
}

// DefaultABCIQueryOptions are latest height (0) and prove equal to true.
var DefaultABCIQueryOptions = ABCIQueryOptions{Height: 0, Trusted: false}
