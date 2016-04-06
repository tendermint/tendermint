package account

import (
	"bytes"
	"testing"

	"github.com/tendermint/ed25519"
	"github.com/eris-ltd/tendermint/wire"
	. "github.com/eris-ltd/tendermint/common"
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
	sigEd := sig.(SignatureEd25519)
	sigEd[0] ^= byte(0x01)
	sig = Signature(sigEd)

	if pubKey.VerifyBytes(msg, sig) {
		t.Errorf("Account message signature verification should have failed but passed instead")
	}
}

func TestBinaryDecode(t *testing.T) {

	privAccount := GenPrivAccount()
	pubKey := privAccount.PubKey
	privKey := privAccount.PrivKey

	msg := CRandBytes(128)
	sig := privKey.Sign(msg)
	t.Logf("msg: %X, sig: %X", msg, sig)

	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	wire.WriteBinary(sig, buf, n, err)
	if *err != nil {
		t.Fatalf("Failed to write Signature: %v", err)
	}

	if len(buf.Bytes()) != ed25519.SignatureSize+1 {
		// 1 byte TypeByte, 64 bytes signature bytes
		t.Fatalf("Unexpected signature write size: %v", len(buf.Bytes()))
	}
	if buf.Bytes()[0] != SignatureTypeEd25519 {
		t.Fatalf("Unexpected signature type byte")
	}

	sig2, ok := wire.ReadBinary(SignatureEd25519{}, buf, n, err).(SignatureEd25519)
	if !ok || *err != nil {
		t.Fatalf("Failed to read Signature: %v", err)
	}

	// Test the signature
	if !pubKey.VerifyBytes(msg, sig2) {
		t.Errorf("Account message signature verification failed")
	}
}
