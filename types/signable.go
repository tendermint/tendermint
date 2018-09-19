package types

// Signable is an interface for all signable things.
// It typically removes signatures before serializing.
// SignBytes returns the bytes to be signed
// NOTE: Expected to panic if there is an error marshalling.
type Signable interface {
	SignBytes() []byte
}
