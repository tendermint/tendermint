package sr25519

import (
	"fmt"

	schnorrkel "github.com/ChainSafe/go-schnorrkel"

	"github.com/tendermint/tendermint/crypto"
)

var _ crypto.BatchVerifier = &BatchVerifier{}

// BatchVerifier implements batch verification for sr25519.
// https://github.com/ChainSafe/go-schnorrkel is used for batch verification
type BatchVerifier struct {
	*schnorrkel.BatchVerifier

	// The go-schnorrkel API is terrible for performance if the chance
	// that a batch fails is non-negligible, exact information about
	// which signature failed is something that is desired, and the
	// system is not resource constrained.
	//
	// The tendermint case meets all of the criteria, so emulate a
	// one-shot with fallback API so that libraries that do provide
	// sensible behavior aren't penalized.
	pubKeys    []crypto.PubKey
	messages   [][]byte
	signatures [][]byte
}

func NewBatchVerifier() crypto.BatchVerifier {
	return &BatchVerifier{schnorrkel.NewBatchVerifier(), nil, nil, nil}
}

func (b *BatchVerifier) Add(key crypto.PubKey, msg, sig []byte) error {
	var sig64 [SignatureSize]byte
	copy(sig64[:], sig)
	signature := new(schnorrkel.Signature)
	err := signature.Decode(sig64)
	if err != nil {
		return fmt.Errorf("unable to decode signature: %w", err)
	}

	signingContext := schnorrkel.NewSigningContext([]byte{}, msg)

	var pk [PubKeySize]byte
	copy(pk[:], key.Bytes())

	err = b.BatchVerifier.Add(signingContext, signature, schnorrkel.NewPublicKey(pk))
	if err == nil {
		b.pubKeys = append(b.pubKeys, key)
		b.messages = append(b.messages, msg)
		b.signatures = append(b.signatures, sig)
	}

	return err
}

func (b *BatchVerifier) Verify() (bool, []bool) {
	// Explicitly mimic ed25519consensus/curve25519-voi behavior.
	if len(b.pubKeys) == 0 {
		return false, nil
	}

	// Optimistically assume everything will succeed.
	valid := make([]bool, len(b.pubKeys))
	for i := range valid {
		valid[i] = true
	}

	// Fast path, every signature may be valid.
	if b.BatchVerifier.Verify() {
		return true, valid
	}

	// Fall-back to serial verification.
	allValid := true
	for i := range b.pubKeys {
		pk, msg, sig := b.pubKeys[i], b.messages[i], b.signatures[i]
		valid[i] = pk.VerifySignature(msg, sig)
		allValid = allValid && valid[i]
	}

	return allValid, valid
}
