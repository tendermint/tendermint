package client

// ABCIQueryOptions can be used to provide options for ABCIQuery call other
// than the DefaultABCIQueryOptions.
type ABCIQueryOptions struct {
	Height uint64
	Prove  bool
}

// DefaultABCIQueryOptions are latest height (0) and prove equal to true.
func DefaultABCIQueryOptions() ABCIQueryOptions {
	return ABCIQueryOptions{Height: 0, Prove: true}
}
