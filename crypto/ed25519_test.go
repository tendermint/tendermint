package crypto

import (
	"crypto/rand"
	"testing"
)

func TestSign(t *testing.T) {
	privKey := make([]byte, 32)
	_, err := rand.Read(privKey)
	if err != nil {
		t.Fatal(err)
	}
	pubKey := MakePubKey(privKey)
	signature := SignMessage([]byte("hello"), privKey, pubKey)

	v1 := &Verify{
		Message:   []byte("hello"),
		PubKey:    pubKey,
		Signature: signature,
	}

	ok := VerifyBatch([]*Verify{v1, v1, v1, v1})
	if ok != true {
		t.Fatal("Expected ok == true")
	}
	if v1.Valid != true {
		t.Fatal("Expected v1.Valid to be true")
	}

	v2 := &Verify{
		Message:   []byte{0x73},
		PubKey:    pubKey,
		Signature: signature,
	}

	ok = VerifyBatch([]*Verify{v1, v1, v1, v2})
	if ok != false {
		t.Fatal("Expected ok == false")
	}
	if v1.Valid != true {
		t.Fatal("Expected v1.Valid to be true")
	}
	if v2.Valid != false {
		t.Fatal("Expected v2.Valid to be true")
	}
}
