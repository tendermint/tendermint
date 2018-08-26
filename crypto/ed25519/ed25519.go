package ed25519

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"io"

	"github.com/tendermint/ed25519"
	"github.com/tendermint/ed25519/extra25519"
	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
)

//-------------------------------------

var _ crypto.PrivKey = PrivKeyEd25519{}

const (
	PrivKeyAminoRoute = "tendermint/PrivKeyEd25519"
	PubKeyAminoRoute  = "tendermint/PubKeyEd25519"
	// Size of an Edwards25519 signature. Namely the size of a compressed
	// Edwards25519 point, and a field element. Both of which are 32 bytes.
	SignatureSize = 64
)

var cdc = amino.NewCodec()

func init() {
	cdc.RegisterInterface((*crypto.PubKey)(nil), nil)
	cdc.RegisterConcrete(PubKeyEd25519{},
		PubKeyAminoRoute, nil)

	cdc.RegisterInterface((*crypto.PrivKey)(nil), nil)
	cdc.RegisterConcrete(PrivKeyEd25519{},
		PrivKeyAminoRoute, nil)
}

// PrivKeyEd25519 implements crypto.PrivKey.
type PrivKeyEd25519 [64]byte

// Bytes marshals the privkey using amino encoding.
func (privKey PrivKeyEd25519) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(privKey)
}

// Sign produces a signature on the provided message.
func (privKey PrivKeyEd25519) Sign(msg []byte) ([]byte, error) {
	privKeyBytes := [64]byte(privKey)
	signatureBytes := ed25519.Sign(&privKeyBytes, msg)
	return signatureBytes[:], nil
}

// PubKey gets the corresponding public key from the private key.
func (privKey PrivKeyEd25519) PubKey() crypto.PubKey {
	privKeyBytes := [64]byte(privKey)
	initialized := false
	// If the latter 32 bytes of the privkey are all zero, compute the pubkey
	// otherwise privkey is initialized and we can use the cached value inside
	// of the private key.
	for _, v := range privKeyBytes[32:] {
		if v != 0 {
			initialized = true
			break
		}
	}
	if initialized {
		var pubkeyBytes [PubKeyEd25519Size]byte
		copy(pubkeyBytes[:], privKeyBytes[32:])
		return PubKeyEd25519(pubkeyBytes)
	}

	pubBytes := *ed25519.MakePublicKey(&privKeyBytes)
	return PubKeyEd25519(pubBytes)
}

// Equals - you probably don't need to use this.
// Runs in constant time based on length of the keys.
func (privKey PrivKeyEd25519) Equals(other crypto.PrivKey) bool {
	if otherEd, ok := other.(PrivKeyEd25519); ok {
		return subtle.ConstantTimeCompare(privKey[:], otherEd[:]) == 1
	} else {
		return false
	}
}

// ToCurve25519 takes a private key and returns its representation on
// Curve25519. Curve25519 is birationally equivalent to Edwards25519,
// which Ed25519 uses internally. This method is intended for use in
// an X25519 Diffie Hellman key exchange.
func (privKey PrivKeyEd25519) ToCurve25519() *[PubKeyEd25519Size]byte {
	keyCurve25519 := new([32]byte)
	privKeyBytes := [64]byte(privKey)
	extra25519.PrivateKeyToCurve25519(keyCurve25519, &privKeyBytes)
	return keyCurve25519
}

// GenPrivKey generates a new ed25519 private key.
// It uses OS randomness in conjunction with the current global random seed
// in tendermint/libs/common to generate the private key.
func GenPrivKey() PrivKeyEd25519 {
	return genPrivKey(crypto.CReader())
}

// genPrivKey generates a new ed25519 private key using the provided reader.
func genPrivKey(rand io.Reader) PrivKeyEd25519 {
	privKey := new([64]byte)
	_, err := io.ReadFull(rand, privKey[:32])
	if err != nil {
		panic(err)
	}
	// ed25519.MakePublicKey(privKey) alters the last 32 bytes of privKey.
	// It places the pubkey in the last 32 bytes of privKey, and returns the
	// public key.
	ed25519.MakePublicKey(privKey)
	return PrivKeyEd25519(*privKey)
}

// GenPrivKeyFromSecret hashes the secret with SHA2, and uses
// that 32 byte output to create the private key.
// NOTE: secret should be the output of a KDF like bcrypt,
// if it's derived from user input.
func GenPrivKeyFromSecret(secret []byte) PrivKeyEd25519 {
	privKey32 := crypto.Sha256(secret) // Not Ripemd160 because we want 32 bytes.
	privKey := new([64]byte)
	copy(privKey[:32], privKey32)
	// ed25519.MakePublicKey(privKey) alters the last 32 bytes of privKey.
	// It places the pubkey in the last 32 bytes of privKey, and returns the
	// public key.
	ed25519.MakePublicKey(privKey)
	return PrivKeyEd25519(*privKey)
}

//-------------------------------------

var _ crypto.PubKey = PubKeyEd25519{}

// PubKeyEd25519Size is the number of bytes in an Ed25519 signature.
const PubKeyEd25519Size = 32

// PubKeyEd25519 implements crypto.PubKey for the Ed25519 signature scheme.
type PubKeyEd25519 [PubKeyEd25519Size]byte

// Address is the SHA256-20 of the raw pubkey bytes.
func (pubKey PubKeyEd25519) Address() crypto.Address {
	return crypto.Address(tmhash.Sum(pubKey[:]))
}

// Bytes marshals the PubKey using amino encoding.
func (pubKey PubKeyEd25519) Bytes() []byte {
	bz, err := cdc.MarshalBinaryBare(pubKey)
	if err != nil {
		panic(err)
	}
	return bz
}

func (pubKey PubKeyEd25519) VerifyBytes(msg []byte, sig_ []byte) bool {
	// make sure we use the same algorithm to sign
	if len(sig_) != SignatureSize {
		return false
	}
	sig := new([SignatureSize]byte)
	copy(sig[:], sig_)
	pubKeyBytes := [PubKeyEd25519Size]byte(pubKey)
	return ed25519.Verify(&pubKeyBytes, msg, sig)
}

// ToCurve25519 takes a public key and returns its representation on
// Curve25519. Curve25519 is birationally equivalent to Edwards25519,
// which Ed25519 uses internally. This method is intended for use in
// an X25519 Diffie Hellman key exchange.
//
// If there is an error, then this function returns nil.
func (pubKey PubKeyEd25519) ToCurve25519() *[PubKeyEd25519Size]byte {
	keyCurve25519, pubKeyBytes := new([PubKeyEd25519Size]byte), [PubKeyEd25519Size]byte(pubKey)
	ok := extra25519.PublicKeyToCurve25519(keyCurve25519, &pubKeyBytes)
	if !ok {
		return nil
	}
	return keyCurve25519
}

func (pubKey PubKeyEd25519) String() string {
	return fmt.Sprintf("PubKeyEd25519{%X}", pubKey[:])
}

// nolint: golint
func (pubKey PubKeyEd25519) Equals(other crypto.PubKey) bool {
	if otherEd, ok := other.(PubKeyEd25519); ok {
		return bytes.Equal(pubKey[:], otherEd[:])
	} else {
		return false
	}
}
