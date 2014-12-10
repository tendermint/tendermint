package account

import (
	. "github.com/tendermint/tendermint/common"
	"testing"
)

func TestSignAndValidate(t *testing.T) {

	privAccount := GenPrivAccount()
	account := &privAccount.Account

	msg := CRandBytes(128)
	sig := privAccount.SignBytes(msg)
	t.Logf("msg: %X, sig: %X", msg, sig)

	// Test the signature
	if !account.VerifyBytes(msg, sig) {
		t.Errorf("Account message signature verification failed")
	}

	// Mutate the signature, just one bit.
	sig.Bytes[0] ^= byte(0x01)

	if account.VerifyBytes(msg, sig) {
		t.Errorf("Account message signature verification should have failed but passed instead")
	}
}
