package bls

import (
	"crypto/rand"
	"testing"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls/blst"
	"github.com/tendermint/tendermint/crypto/bls/ostracon"
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

func TestBlstSignVerify(t *testing.T) {
	m := []byte("a test message to be signed")
	privKey := blst.GenPrivKey()
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

func TestOstraconSignVerify(t *testing.T) {
	m := []byte("a test message to be signed")
	privKey := ostracon.GenPrivKey()
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

// Retrieve a set of public keys from a set of private keys.
func pubKeysFromPrivKeys(privKeys []crypto.PrivKey) []crypto.PubKey {
	pubKeys := make([]crypto.PubKey, len(privKeys))
	for i := 0; i < len(privKeys); i++ {
		pubKeys[i] = privKeys[i].PubKey()
	}
	return pubKeys
}

// Generate n random messages with the given size.
func randomMessages(n, size int) [][]byte {
	messages := make([][]byte, n)
	for i := 0; i < n; i++ {
		messages[i] = make([]byte, size)
		rand.Read(messages[i])
	}
	return messages
}

// Sign a set of messages with a number of private keys.
func signMessages(messages [][]byte, privKeys []crypto.PrivKey) [][]byte {
	sigs := make([][]byte, len(messages))
	for i := 0; i < len(messages); i++ {
		sigs[i], _ = privKeys[i%len(privKeys)].Sign(messages[i])
	}
	return sigs
}

var _sig []byte

func BenchmarkEd25519Sign1k(t *testing.B) {
	messages := randomMessages(1000, 1024)
	privKeys := make([]crypto.PrivKey, 128)
	for i := 0; i < len(privKeys); i++ {
		privKeys[i] = ed25519.GenPrivKey()
	}
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		_sig, _ = privKeys[i%len(privKeys)].Sign(messages[i%len(messages)])
	}
}

func BenchmarkEd25519Verify1k(t *testing.B) {
	messages := randomMessages(1000, 1024)
	privKeys := make([]crypto.PrivKey, 128)
	for i := 0; i < len(privKeys); i++ {
		privKeys[i] = ed25519.GenPrivKey()
	}
	pubKeys := pubKeysFromPrivKeys(privKeys)
	sigs := signMessages(messages, privKeys)
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		pubKeys[i%len(pubKeys)].VerifySignature(messages[i%len(messages)], sigs[i%len(sigs)])
	}
}

func BenchmarkBlstSign1k(t *testing.B) {
	messages := randomMessages(1000, 1024)
	privKeys := make([]crypto.PrivKey, 128)
	for i := 0; i < len(privKeys); i++ {
		privKeys[i] = blst.GenPrivKey()
	}
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		_sig, _ = privKeys[i%len(privKeys)].Sign(messages[i%len(messages)])
	}
}

func BenchmarkBlstVerify1k(t *testing.B) {
	messages := randomMessages(1000, 1024)
	privKeys := make([]crypto.PrivKey, 128)
	for i := 0; i < len(privKeys); i++ {
		privKeys[i] = blst.GenPrivKey()
	}
	pubKeys := pubKeysFromPrivKeys(privKeys)
	sigs := signMessages(messages, privKeys)
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		pubKeys[i%len(pubKeys)].VerifySignature(messages[i%len(messages)], sigs[i%len(sigs)])
	}
}

func BenchmarkOstraconSign1k(t *testing.B) {
	messages := randomMessages(1000, 1024)
	privKeys := make([]crypto.PrivKey, 128)
	for i := 0; i < len(privKeys); i++ {
		privKeys[i] = ostracon.GenPrivKey()
	}
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		_sig, _ = privKeys[i%len(privKeys)].Sign(messages[i%len(messages)])
	}
}

func BenchmarkOstraconVerify1k(t *testing.B) {
	messages := randomMessages(1000, 1024)
	privKeys := make([]crypto.PrivKey, 128)
	for i := 0; i < len(privKeys); i++ {
		privKeys[i] = ostracon.GenPrivKey()
	}
	pubKeys := pubKeysFromPrivKeys(privKeys)
	sigs := signMessages(messages, privKeys)
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		pubKeys[i%len(pubKeys)].VerifySignature(messages[i%len(messages)], sigs[i%len(sigs)])
	}
}
