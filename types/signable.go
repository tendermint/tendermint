package types

import (
	"github.com/dashevo/dashd-go/btcjson"
	"github.com/tendermint/tendermint/crypto/bls12381"
)

var (
	SignatureSize = bls12381.SignatureSize
)

// Signable is an interface for all signable things.
// It typically removes signatures before serializing.
// NOTE: chainIDs are part of the SignBytes but not
// necessarily the object themselves.
// NOTE: Expected to panic if there is an error marshaling.
type Signable interface {
	// SignBytes returns the bytes to be signed.
	SignBytes(chainID string) []byte
	// SignRequestID returns a deterministically calculable, unique id of a signing request.
	// See DIP-0007 for more details.
	SignRequestID() []byte

	// SignID returns signing session data that will be signed to get threshold signature share.
	// See DIP-0007 for more details.
	SignID(chainID string, quorumType btcjson.LLMQType, quorumHash []byte) []byte
}
