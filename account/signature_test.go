package account

import (
	. "github.com/tendermint/tendermint/common"
	"testing"
)

func TestSignAndValidate(t *testing.T) {

	privAccount := GenPrivAccount()
	pubKey := privAccount.PubKey
	privKey := privAccount.PrivKey

	msg := CRandBytes(128)
	sig := privKey.Sign(msg)
	t.Logf("msg: %X, sig: %X", msg, sig)

	// Test the signature
	if !pubKey.VerifyBytes(msg, sig) {
		t.Errorf("Account message signature verification failed")
	}

	// Mutate the signature, just one bit.
	sig.(SignatureEd25519).Bytes[0] ^= byte(0x01)

	if pubKey.VerifyBytes(msg, sig) {
		t.Errorf("Account message signature verification should have failed but passed instead")
	}
}
