package types

const (
	// MaxSignatureSize is a maximum allowed signature size for the Heartbeat,
	// Proposal and Vote.
	MaxSignatureSize = 64
)

// Signable is an interface for all signable things.
// It typically removes signatures before serializing.
// SignBytes returns the bytes to be signed
// NOTE: chainIDs are part of the SignBytes but not
// necessarily the object themselves.
// NOTE: Expected to panic if there is an error marshalling.
type Signable interface {
	SignBytes(chainID string) []byte
}
