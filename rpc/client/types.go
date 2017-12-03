package client

// ABCIQueryOptions can be used to provide options for ABCIQuery call other
// than the DefaultABCIQueryOptions.
type ABCIQueryOptions struct {
	Height  int64
	Trusted bool
}

// DefaultABCIQueryOptions are latest height (0) and trusted equal to false
// (which will result in a proof being returned).
var DefaultABCIQueryOptions = ABCIQueryOptions{Height: 0, Trusted: false}
