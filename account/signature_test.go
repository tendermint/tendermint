package account

import (
	"bytes"
	"testing"

	"github.com/tendermint/ed25519"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
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
	sig.(SignatureEd25519)[0] ^= byte(0x01)

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
	binary.WriteBinary(sig, buf, n, err)
	if *err != nil {
		t.Fatalf("Failed to write Signature: %v", err)
	}

	if len(buf.Bytes()) != ed25519.SignatureSize+2 {
		// 1 byte TypeByte, 1 byte length, 64 bytes signature bytes
		t.Fatalf("Unexpected signature write size: %v", len(buf.Bytes()))
	}
	if buf.Bytes()[0] != SignatureTypeEd25519 {
		t.Fatalf("Unexpected signature type byte")
	}

	sig2, ok := binary.ReadBinary(SignatureEd25519{}, buf, n, err).(SignatureEd25519)
	if !ok || *err != nil {
		t.Fatalf("Failed to read Signature: %v", err)
	}

	// Test the signature
	if !pubKey.VerifyBytes(msg, sig2) {
		t.Errorf("Account message signature verification failed")
	}
}
