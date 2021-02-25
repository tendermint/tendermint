package sr25519

import (
	"errors"

	schnorrkel "github.com/ChainSafe/go-schnorrkel"
	"github.com/tendermint/tendermint/crypto"
)

var _ crypto.BatchVerifier = BatchVerifier{}

type BatchVerifier struct {
	*schnorrkel.BatchVerifier
}

func NewBatchVerifier() crypto.BatchVerifier {
	return BatchVerifier{schnorrkel.NewBatchVerifier()}
}

func (b BatchVerifier) Add(key crypto.PubKey, msg, sig []byte) error {
	var sig64 [SignatureSize]byte
	copy(sig64[:], sig)
	signature := &(schnorrkel.Signature{})
	err := signature.Decode(sig64)
	if err != nil {
		return errors.New("unable to decode signature")
	}

	signingContext := schnorrkel.NewSigningContext([]byte{}, msg)

	var pk [PubKeySize]byte
	copy(pk[:], key.Bytes())

	err = b.BatchVerifier.Add(signingContext, signature, schnorrkel.NewPublicKey(pk))

	return err
}

func (b BatchVerifier) Verify() bool {
	b.BatchVerifier.Verify()
	return true
}
