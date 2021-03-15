package sr25519

import (
	"fmt"

	schnorrkel "github.com/ChainSafe/go-schnorrkel"

	"github.com/tendermint/tendermint/crypto"
)

var _ crypto.BatchVerifier = BatchVerifier{}

// BatchVerifier implements batch verification for sr25519.
// https://github.com/ChainSafe/go-schnorrkel is used for batch verification
type BatchVerifier struct {
	*schnorrkel.BatchVerifier
}

func NewBatchVerifier() crypto.BatchVerifier {
	return BatchVerifier{schnorrkel.NewBatchVerifier()}
}

func (b BatchVerifier) Add(key crypto.PubKey, msg, sig []byte) error {
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

	return b.BatchVerifier.Add(signingContext, signature, schnorrkel.NewPublicKey(pk))
}

func (b BatchVerifier) Verify() bool {
	return b.BatchVerifier.Verify()
}
