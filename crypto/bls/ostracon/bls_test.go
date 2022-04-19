package bls_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	b "github.com/herumi/bls-eth-go-binary/bls"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/line/ostracon/crypto"
	"github.com/line/ostracon/crypto/bls"
)

func TestBasicSignatureFunctions(t *testing.T) {
	privateKey := b.SecretKey{}
	privateKey.SetByCSPRNG()
	publicKey := privateKey.GetPublicKey()

	duplicatedPrivateKey := b.SecretKey{}
	err := duplicatedPrivateKey.Deserialize(privateKey.Serialize())
	if err != nil {
		t.Fatalf("Private key deserialization failed.")
	}

	if len(privateKey.Serialize()) != bls.PrivKeySize {
		t.Fatalf("The constant size %d of the private key is different from the actual size %d.",
			bls.PrivKeySize, len(privateKey.Serialize()))
	}

	duplicatedPublicKey := b.PublicKey{}
	err = duplicatedPublicKey.Deserialize(publicKey.Serialize())
	if err != nil {
		t.Fatalf("Public key deserialization failed.")
	}

	if len(publicKey.Serialize()) != bls.PubKeySize {
		t.Fatalf("The constant size %d of the public key is different from the actual size %d.",
			bls.PubKeySize, len(publicKey.Serialize()))
	}

	duplicatedSignature := func(sig *b.Sign) *b.Sign {
		duplicatedSign := b.Sign{}
		err := duplicatedSign.Deserialize(sig.Serialize())
		if err != nil {
			t.Fatalf("Signature deserialization failed.")
		}

		if len(sig.Serialize()) != bls.SignatureSize {
			t.Fatalf("The constant size %d of the signature is different from the actual size %d.",
				bls.SignatureSize, len(sig.Serialize()))
		}
		return &duplicatedSign
	}

	msg := []byte("hello, world")
	for _, privKey := range []b.SecretKey{privateKey, duplicatedPrivateKey} {
		for _, pubKey := range []*b.PublicKey{publicKey, &duplicatedPublicKey} {
			signature := privKey.SignByte(msg)
			if !signature.VerifyByte(pubKey, msg) {
				t.Errorf("Signature verification failed.")
			}

			if !duplicatedSignature(signature).VerifyByte(pubKey, msg) {
				t.Errorf("Signature verification failed.")
			}

			for i := 0; i < len(msg); i++ {
				for j := 0; j < 8; j++ {
					garbled := make([]byte, len(msg))
					copy(garbled, msg)
					garbled[i] ^= 1 << (8 - j - 1)
					if bytes.Equal(msg, garbled) {
						t.Fatalf("Not a barbled message")
					}
					if signature.VerifyByte(pubKey, garbled) {
						t.Errorf("Signature verification was successful against a garbled byte sequence.")
					}
					if duplicatedSignature(signature).VerifyByte(pubKey, garbled) {
						t.Errorf("Signature verification was successful against a garbled byte sequence.")
					}
				}
			}
		}
	}
}

func TestSignatureAggregationAndVerify(t *testing.T) {
	privKeys := make([]b.SecretKey, 25)
	pubKeys := make([]b.PublicKey, len(privKeys))
	msgs := make([][]byte, len(privKeys))
	hash32s := make([][32]byte, len(privKeys))
	signatures := make([]b.Sign, len(privKeys))
	for i, privKey := range privKeys {
		privKey.SetByCSPRNG()
		pubKeys[i] = *privKey.GetPublicKey()
		msgs[i] = []byte(fmt.Sprintf("hello, world #%d", i))
		hash32s[i] = sha256.Sum256(msgs[i])
		signatures[i] = *privKey.SignHash(hash32s[i][:])

		// normal single-hash case
		if !signatures[i].VerifyHash(&pubKeys[i], hash32s[i][:]) {
			t.Fail()
		}

		// in case where 1-bit of hash was garbled
		garbledHash := make([]byte, len(msgs[i]))
		copy(garbledHash, msgs[i])
		garbledHash[0] ^= 1 << 0
		if garbledHash[0] == msgs[i][0] || signatures[i].VerifyByte(&pubKeys[i], garbledHash) {
			t.Fail()
		}

		// Verification using an invalid public key
	}

	// aggregation
	multiSig := b.Sign{}
	multiSig.Aggregate(signatures)

	// normal case
	hashes := make([][]byte, len(privKeys))
	for i := 0; i < len(hashes); i++ {
		hashes[i] = hash32s[i][:]
	}
	if !multiSig.VerifyAggregateHashes(pubKeys, hashes) {
		t.Fatalf("failed to validate the aggregate signature of the hashed message")
	}

	// in case where 1-bit of signature was garbled
	multiSigBytes := multiSig.Serialize()
	for i := range multiSigBytes {
		for j := 0; j < 8; j++ {
			garbledMultiSigBytes := make([]byte, len(multiSigBytes))
			copy(garbledMultiSigBytes, multiSigBytes)
			garbledMultiSigBytes[i] ^= 1 << j
			if garbledMultiSigBytes[i] == multiSigBytes[i] {
				t.Fail()
			}
			garbledMultiSig := b.Sign{}
			err := garbledMultiSig.Deserialize(garbledMultiSigBytes)
			if err == nil {
				// Note that in some cases Deserialize() fails
				if garbledMultiSig.VerifyAggregateHashes(pubKeys, hashes) {
					t.Errorf("successfully verified the redacted signature")
				}
			}
		}
	}

	// in case a public key used for verification is replaced
	invalidPrivKey := b.SecretKey{}
	invalidPrivKey.SetByCSPRNG()
	invalidPubKeys := make([]b.PublicKey, len(pubKeys))
	copy(invalidPubKeys, pubKeys)
	invalidPubKeys[len(invalidPubKeys)-1] = *invalidPrivKey.GetPublicKey()
	if multiSig.VerifyAggregateHashes(invalidPubKeys, hashes) {
		t.Fatalf("successfully verified that it contains a public key that was not involved in the signing")
	}

	// in case a hash used for verification is replaced
	invalidHashes := make([][]byte, len(hashes))
	copy(invalidHashes, hashes)
	invalidHash := sha256.Sum256([]byte("hello, world #99"))
	invalidHashes[len(invalidHashes)-1] = invalidHash[:]
	if multiSig.VerifyAggregateHashes(pubKeys, invalidHashes) {
		t.Fatalf("successfully verified that it contains a hash that was not involved in the signing")
	}
}

func generatePubKeysAndSigns(t *testing.T, size int) ([]bls.PubKey, [][]byte, [][]byte) {
	pubKeys := make([]bls.PubKey, size)
	msgs := make([][]byte, len(pubKeys))
	sigs := make([][]byte, len(pubKeys))
	for i := 0; i < len(pubKeys); i++ {
		var err error
		privKey := bls.GenPrivKey()
		pubKeys[i] = blsPublicKey(t, privKey.PubKey())
		msgs[i] = []byte(fmt.Sprintf("hello, workd #%d", i))
		sigs[i], err = privKey.Sign(msgs[i])
		if err != nil {
			t.Fatal(fmt.Sprintf("fail to sign: %s", err))
		}
		if !pubKeys[i].VerifySignature(msgs[i], sigs[i]) {
			t.Fatal("fail to verify signature")
		}
	}
	return pubKeys, msgs, sigs
}

func blsPublicKey(t *testing.T, pubKey crypto.PubKey) bls.PubKey {
	blsPubKey, ok := pubKey.(bls.PubKey)
	if !ok {
		var keyType string
		if t := reflect.TypeOf(pubKey); t.Kind() == reflect.Ptr {
			keyType = "*" + t.Elem().Name()
		} else {
			keyType = t.Name()
		}
		t.Fatal(fmt.Sprintf("specified public key is not for BLS: %s", keyType))
	}
	return blsPubKey
}

func aggregateSignatures(init []byte, signatures [][]byte) (aggrSig []byte, err error) {
	aggrSig = init
	for _, sign := range signatures {
		aggrSig, err = bls.AddSignature(aggrSig, sign)
		if err != nil {
			return
		}
	}
	return
}

func TestAggregatedSignature(t *testing.T) {

	// generate BLS signatures and public keys
	pubKeys, msgs, sigs := generatePubKeysAndSigns(t, 25)

	// aggregate signatures
	aggrSig, err := aggregateSignatures(nil, sigs)
	if err != nil {
		t.Errorf("fail to aggregate BLS signatures: %s", err)
	}
	if len(aggrSig) != bls.SignatureSize {
		t.Errorf("inconpatible signature size: %d != %d", len(aggrSig), bls.SignatureSize)
	}

	// validate the aggregated signature
	if err := bls.VerifyAggregatedSignature(aggrSig, pubKeys, msgs); err != nil {
		t.Errorf("fail to verify aggregated signature: %s", err)
	}

	// validate with the public keys and messages pair in random order
	t.Run("Doesn't Depend on the Order of PublicKey-Message Pairs", func(t *testing.T) {
		shuffledPubKeys := make([]bls.PubKey, len(pubKeys))
		shuffledMsgs := make([][]byte, len(msgs))
		copy(shuffledPubKeys, pubKeys)
		copy(shuffledMsgs, msgs)
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(shuffledPubKeys), func(i, j int) {
			shuffledPubKeys[i], shuffledPubKeys[j] = shuffledPubKeys[j], shuffledPubKeys[i]
			shuffledMsgs[i], shuffledMsgs[j] = shuffledMsgs[j], shuffledMsgs[i]
		})
		if err := bls.VerifyAggregatedSignature(aggrSig, shuffledPubKeys, shuffledMsgs); err != nil {
			t.Errorf("fail to verify the aggregated signature with random order: %s", err)
		}
	})

	// validate with the public keys in random order
	t.Run("Incorrect Public Key Order", func(t *testing.T) {
		shuffledPubKeys := make([]bls.PubKey, len(pubKeys))
		copy(shuffledPubKeys, pubKeys)
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(shuffledPubKeys), func(i, j int) {
			shuffledPubKeys[i], shuffledPubKeys[j] = shuffledPubKeys[j], shuffledPubKeys[i]
		})
		if err := bls.VerifyAggregatedSignature(aggrSig, shuffledPubKeys, msgs); err == nil {
			t.Error("successfully validated with public keys of different order")
		}
	})

	// validate with the messages in random order
	t.Run("Incorrect Message Order", func(t *testing.T) {
		shuffledMsgs := make([][]byte, len(msgs))
		copy(shuffledMsgs, msgs)
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(shuffledMsgs), func(i, j int) {
			shuffledMsgs[i], shuffledMsgs[j] = shuffledMsgs[j], shuffledMsgs[i]
		})
		if err := bls.VerifyAggregatedSignature(aggrSig, pubKeys, shuffledMsgs); err == nil {
			t.Error("successfully validated with messages of different order")
		}
	})

	// replace one public key with another and detect
	t.Run("Replace One Public Key", func(t *testing.T) {
		pubKey, _ := bls.GenPrivKey().PubKey().(bls.PubKey)
		replacedPubKeys := make([]bls.PubKey, len(pubKeys))
		copy(replacedPubKeys, pubKeys)
		replacedPubKeys[0] = pubKey
		if err := bls.VerifyAggregatedSignature(aggrSig, replacedPubKeys, msgs); err == nil {
			t.Error("verification with an invalid key was successful")
		}
	})

	// replace one message with another and detect
	t.Run("Replace One Message", func(t *testing.T) {
		msg := []byte(fmt.Sprintf("hello, world #%d replaced", len(msgs)))
		replacedMsgs := make([][]byte, len(msgs))
		copy(replacedMsgs, msgs)
		replacedMsgs[0] = msg
		if err := bls.VerifyAggregatedSignature(aggrSig, pubKeys, replacedMsgs); err == nil {
			t.Error("verification with an invalid message was successful")
		}
	})

	// add new signature to existing aggregated signature and verify
	t.Run("Incremental Update", func(t *testing.T) {
		msg := []byte(fmt.Sprintf("hello, world #%d", len(msgs)))
		privKey := bls.GenPrivKey()
		pubKey := privKey.PubKey()
		sig, err := privKey.Sign(msg)
		assert.Nilf(t, err, "%s", err)
		newAggrSig, _ := aggregateSignatures(aggrSig, [][]byte{sig})
		newPubKeys := make([]bls.PubKey, len(pubKeys))
		copy(newPubKeys, pubKeys)
		newPubKeys = append(newPubKeys, blsPublicKey(t, pubKey))
		newMsgs := make([][]byte, len(msgs))
		copy(newMsgs, msgs)
		newMsgs = append(newMsgs, msg)
		if err := bls.VerifyAggregatedSignature(newAggrSig, newPubKeys, newMsgs); err != nil {
			t.Errorf("fail to verify the aggregate signature with the new signature: %s", err)
		}
	})

	// nil is returned for nil and empty signature
	nilSig, _ := aggregateSignatures(nil, [][]byte{})
	assert.Nil(t, nilSig)

	// a non-BLS signature contained
	func() {
		_, err = aggregateSignatures(nil, [][]byte{make([]byte, 0)})
		assert.NotNil(t, err)
	}()
}

func TestSignatureAggregation(t *testing.T) {
	publicKeys := make([]b.PublicKey, 25)
	aggregatedSignature := b.Sign{}
	aggregatedPublicKey := b.PublicKey{}
	msg := []byte("hello, world")
	for i := 0; i < len(publicKeys); i++ {
		privateKey := b.SecretKey{}
		privateKey.SetByCSPRNG()
		publicKeys[i] = *privateKey.GetPublicKey()
		aggregatedSignature.Add(privateKey.SignByte(msg))
		aggregatedPublicKey.Add(&publicKeys[i])
	}

	if !aggregatedSignature.FastAggregateVerify(publicKeys, msg) {
		t.Errorf("Aggregated signature verification failed.")
	}
	if !aggregatedSignature.VerifyByte(&aggregatedPublicKey, msg) {
		t.Errorf("Aggregated signature verification failed.")
	}
}

func TestSignAndValidateBLS12(t *testing.T) {
	privKey := bls.GenPrivKey()
	pubKey := privKey.PubKey()

	msg := crypto.CRandBytes(128)
	sig, err := privKey.Sign(msg)
	require.Nil(t, err)
	fmt.Printf("restoring signature: %x\n", sig)

	// Test the signature
	assert.True(t, pubKey.VerifySignature(msg, sig))

	// Mutate the signature, just one bit.
	// TODO: Replace this with a much better fuzzer, tendermint/ed25519/issues/10
	sig[7] ^= byte(0x01)

	assert.False(t, pubKey.VerifySignature(msg, sig))
}
