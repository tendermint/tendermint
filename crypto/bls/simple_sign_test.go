package bls

import (
	"testing"

	"github.com/tendermint/tendermint/crypto/ed25519"
)

func TestEd25519SignVerify(t *testing.T) {
	m := []byte("a test message to be signed")
	privKey := ed25519.GenPrivKey()
	pubKey := privKey.PubKey()

	sig, err := privKey.Sign(m)
	if err != nil {
		t.Error("Unexpected nil signature or error", m, sig, err)
	}
	if !pubKey.VerifySignature(m, sig) {
		t.Error("Failed to verify signature produced by the key", m, sig)
	}
	if pubKey.VerifySignature(scrambleBytes(m), sig) {
		t.Error("Unexpected to verify scrambled message", scrambleBytes(m), sig)
	}
	if pubKey.VerifySignature(m, scrambleBytes(sig)) {
		t.Error("Unexpected to verify scrambled signature", m, scrambleBytes(sig))
	}
}

// Scrambles a byte array, currently just flipping a bit.
// TODO: implement a more complex scrambling method.
func scrambleBytes(b []byte) []byte {
	bb := make([]byte, len(b))
	copy(bb, b)
	index := len(b) / 2
	bb[index] ^= 1
	return bb
}
