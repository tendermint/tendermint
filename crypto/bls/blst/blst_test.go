package blst

import "testing"

func TestKeyGen(t *testing.T) {
	key1 := GenPrivKey()
	if key1 == nil {
		t.Fatal("GenPrivKey() return nil")
	}
	b := key1.Bytes()
	if len(b) != PrivKeySize {
		t.Error("Serilalized private key", len(b), b)
	}

	key2 := GenPrivKey()
	if key2 == nil {
		t.Fatal("GenPrivKey() return nil")
	}
	b = key2.Bytes()
	if len(b) != PrivKeySize {
		t.Error("Serilalized private key", len(b), b)
	}

	if !key1.Equals(key1) {
		t.Error("Private key not equal to itself", key1)
	}
	if !key2.Equals(key2) {
		t.Error("Private key not equal to itself", key1)
	}
	if key1.Equals(key2) || key2.Equals(key1) {
		t.Error("Different private keys equal", key1, key2)
	}

	pub1 := key1.PubKey()
	if pub1 == nil {
		t.Fatal("PubKey() return nil")
	}
	b = pub1.Bytes()
	if len(b) != PubKeySize {
		t.Error("Serilalized public key", len(b), b)
	}

	pub2 := key2.PubKey()
	if pub2 == nil {
		t.Fatal("PubKey() return nil")
	}
	b = pub2.Bytes()
	if len(b) != PubKeySize {
		t.Error("Serilalized public key", len(b), b)
	}

	if !pub1.Equals(pub1) {
		t.Error("Public key not equal to itself", pub1)
	}
	if !pub2.Equals(pub2) {
		t.Error("Public key not equal to itself", pub1)
	}
	if pub1.Equals(pub2) || pub2.Equals(pub1) {
		t.Error("Different public keys equal", pub1, pub2)
	}
}

func TestKeySignVerify(t *testing.T) {
	msg := []byte("a test message")
	key1 := GenPrivKey()
	sig1, err := key1.Sign(msg)
	if sig1 == nil || err != nil {
		t.Fatal("Failed to sign message", sig1, err)
	}
	if len(sig1) != SignatureSize {
		t.Error("Signature size", len(sig1), sig1)
	}

	key2 := GenPrivKey()
	sig2, err := key2.Sign(msg)
	if sig2 == nil || err != nil {
		t.Fatal("Failed to sign message", sig2, err)
	}
	if len(sig2) != SignatureSize {
		t.Error("Signature size", len(sig2), sig2)
	}

	pub1 := key1.PubKey()
	pub2 := key2.PubKey()
	if !pub1.VerifySignature(msg, sig1) {
		t.Error("Failed to verify own signature", msg, sig1)
	}
	if !pub2.VerifySignature(msg, sig2) {
		t.Error("Failed to verify own signature", msg, sig2)
	}
	if pub1.VerifySignature(msg, sig2) {
		t.Error("Signature verified by wrong key", msg, sig2)
	}
	if pub2.VerifySignature(msg, sig1) {
		t.Error("Signature verified by wrong key", msg, sig1)
	}
}
