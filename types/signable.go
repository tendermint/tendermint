package types

// Signable is an interface for all signable things.
// It typically removes signatures before serializing.
// SignBytes returns the bytes to be signed
// NOTE: chainIDs are part of the SignBytes but not
// necessarily the object themselves.
// NOTE: Expected to panic if there is an error marshalling.
type Signable interface {
	SignBytes(chainID string) []byte
}

// HashSignBytes is a convenience method for getting the hash of the bytes of a signable
func HashSignBytes(chainID string, o Signable) []byte {
	return tmHash(o.SignBytes(chainID))
}
