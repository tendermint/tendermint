package sr25519

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"

	"github.com/oasisprotocol/curve25519-voi/primitives/sr25519"

	"github.com/tendermint/tendermint/crypto"
)

var (
	_ crypto.PrivKey = PrivKey{}

	signingCtx = sr25519.NewSigningContext([]byte{})
)

const (
	// PrivKeySize is the size of a sr25519 signature in bytes.
	PrivKeySize = sr25519.MiniSecretKeySize

	KeyType = "sr25519"
)

// PrivKey implements crypto.PrivKey.
type PrivKey struct {
	msk sr25519.MiniSecretKey
	kp  *sr25519.KeyPair
}

// TypeTag satisfies the jsontypes.Tagged interface.
func (PrivKey) TypeTag() string { return PrivKeyName }

// Bytes returns the byte-encoded PrivKey.
func (privKey PrivKey) Bytes() []byte {
	if privKey.kp == nil {
		return nil
	}
	return privKey.msk[:]
}

// Sign produces a signature on the provided message.
func (privKey PrivKey) Sign(msg []byte) ([]byte, error) {
	if privKey.kp == nil {
		return nil, fmt.Errorf("sr25519: uninitialized private key")
	}

	st := signingCtx.NewTranscriptBytes(msg)

	sig, err := privKey.kp.Sign(rand.Reader, st)
	if err != nil {
		return nil, fmt.Errorf("sr25519: failed to sign message: %w", err)
	}

	sigBytes, err := sig.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("sr25519: failed to serialize signature: %w", err)
	}

	return sigBytes, nil
}

// PubKey gets the corresponding public key from the private key.
//
// Panics if the private key is not initialized.
func (privKey PrivKey) PubKey() crypto.PubKey {
	if privKey.kp == nil {
		panic("sr25519: uninitialized private key")
	}

	b, err := privKey.kp.PublicKey().MarshalBinary()
	if err != nil {
		panic("sr25519: failed to serialize public key: " + err.Error())
	}

	return PubKey(b)
}

// Equals - you probably don't need to use this.
// Runs in constant time based on length of the keys.
func (privKey PrivKey) Equals(other crypto.PrivKey) bool {
	if otherSr, ok := other.(PrivKey); ok {
		return privKey.msk.Equal(&otherSr.msk)
	}
	return false
}

func (privKey PrivKey) Type() string {
	return KeyType
}

func (privKey PrivKey) MarshalJSON() ([]byte, error) {
	var b []byte

	// Handle uninitialized private keys gracefully.
	if privKey.kp != nil {
		b = privKey.Bytes()
	}

	return json.Marshal(b)
}

func (privKey *PrivKey) UnmarshalJSON(data []byte) error {
	for i := range privKey.msk {
		privKey.msk[i] = 0
	}
	privKey.kp = nil

	var b []byte
	if err := json.Unmarshal(data, &b); err != nil {
		return fmt.Errorf("sr25519: failed to deserialize JSON: %w", err)
	}
	if len(b) == 0 {
		return nil
	}

	msk, err := sr25519.NewMiniSecretKeyFromBytes(b)
	if err != nil {
		return err
	}

	sk := msk.ExpandEd25519()

	privKey.msk = *msk
	privKey.kp = sk.KeyPair()

	return nil
}

// GenPrivKey generates a new sr25519 private key.
// It uses OS randomness in conjunction with the current global random seed
// in tendermint/libs/common to generate the private key.
func GenPrivKey() PrivKey {
	return genPrivKey(rand.Reader)
}

func genPrivKey(rng io.Reader) PrivKey {
	msk, err := sr25519.GenerateMiniSecretKey(rng)
	if err != nil {
		panic("sr25519: failed to generate MiniSecretKey: " + err.Error())
	}

	sk := msk.ExpandEd25519()

	return PrivKey{
		msk: *msk,
		kp:  sk.KeyPair(),
	}
}

// GenPrivKeyFromSecret hashes the secret with SHA2, and uses
// that 32 byte output to create the private key.
// NOTE: secret should be the output of a KDF like bcrypt,
// if it's derived from user input.
func GenPrivKeyFromSecret(secret []byte) PrivKey {
	seed := sha256.Sum256(secret)
	var privKey PrivKey
	if err := privKey.msk.UnmarshalBinary(seed[:]); err != nil {
		panic("sr25519: failed to deserialize MiniSecretKey: " + err.Error())
	}

	sk := privKey.msk.ExpandEd25519()
	privKey.kp = sk.KeyPair()

	return privKey
}
