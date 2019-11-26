package sr25519

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"io"

	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"

	schnorrkel "github.com/ChainSafe/go-schnorrkel"
)

//-------------------------------------

var _ crypto.PrivKey = PrivKeySr25519{}

const (
	PrivKeyAminoName = "tendermint/PrivKeySr25519"
	PubKeyAminoName  = "tendermint/PubKeySr25519"

	// SignatureSize is the size of an Edwards25519 signature. Namely the size of a compressed
	// Sr25519 point, and a field element. Both of which are 32 bytes.
	SignatureSize = 64
)

var cdc = amino.NewCodec()

func init() {
	cdc.RegisterInterface((*crypto.PubKey)(nil), nil)
	cdc.RegisterConcrete(PubKeySr25519{},
		PubKeyAminoName, nil)

	cdc.RegisterInterface((*crypto.PrivKey)(nil), nil)
	cdc.RegisterConcrete(PrivKeySr25519{},
		PrivKeyAminoName, nil)
}

// PrivKeySr25519 implements crypto.PrivKey.
type PrivKeySr25519 [32]byte

// Bytes marshals the privkey using amino encoding.
func (privKey PrivKeySr25519) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(privKey)
}

// Sign produces a signature on the provided message.
func (privKey PrivKeySr25519) Sign(msg []byte) ([]byte, error) {
	secretKey := &(schnorrkel.SecretKey{})
	err := secretKey.Decode(privKey)
	if err != nil {
		return []byte{}, err
	}

	signingContext := schnorrkel.NewSigningContext([]byte{}, msg)

	sig, err := secretKey.Sign(signingContext)
	if err != nil {
		return []byte{}, err
	}

	sigBytes := sig.Encode()
	return sigBytes[:], nil
}

// PubKey gets the corresponding public key from the private key.
func (privKey PrivKeySr25519) PubKey() crypto.PubKey {

	secretKey := &(schnorrkel.SecretKey{})
	err := secretKey.Decode(privKey)
	if err != nil {
		panic("invalid private key")
	}

	pubkey, _ := secretKey.Public()

	return PubKeySr25519(pubkey.Encode())
}

// Equals - you probably don't need to use this.
// Runs in constant time based on length of the keys.
func (privKey PrivKeySr25519) Equals(other crypto.PrivKey) bool {
	if otherEd, ok := other.(PrivKeySr25519); ok {
		return subtle.ConstantTimeCompare(privKey[:], otherEd[:]) == 1
	} else {
		return false
	}
}

// GenPrivKey generates a new sr25519 private key.
// It uses OS randomness in conjunction with the current global random seed
// in tendermint/libs/common to generate the private key.
func GenPrivKey() PrivKeySr25519 {
	return genPrivKey(crypto.CReader())
}

// genPrivKey generates a new ed25519 private key using the provided reader.
func genPrivKey(rand io.Reader) PrivKeySr25519 {
	var seed [64]byte

	out := make([]byte, 64)
	_, err := io.ReadFull(rand, out)
	if err != nil {
		panic(err)
	}

	copy(seed[:], out)

	return schnorrkel.NewMiniSecretKey(seed).ExpandEd25519().Encode()
}

// GenPrivKeyFromSecret hashes the secret with SHA2, and uses
// that 32 byte output to create the private key.
// NOTE: secret should be the output of a KDF like bcrypt,
// if it's derived from user input.
func GenPrivKeyFromSecret(secret []byte) PrivKeySr25519 {
	seed := crypto.Sha256(secret) // Not Ripemd160 because we want 32 bytes.
	var bz [32]byte
	copy(bz[:], seed)
	privKey, _ := schnorrkel.NewMiniSecretKeyFromRaw(bz)
	return privKey.ExpandEd25519().Encode()
}

//-------------------------------------

var _ crypto.PubKey = PubKeySr25519{}

// PubKeyEd25519Size is the number of bytes in an Ed25519 signature.
const PubKeySr25519Size = 32

// PubKeyEd25519 implements crypto.PubKey for the Ed25519 signature scheme.
type PubKeySr25519 [PubKeySr25519Size]byte

// Address is the SHA256-20 of the raw pubkey bytes.
func (pubKey PubKeySr25519) Address() crypto.Address {
	return crypto.Address(tmhash.SumTruncated(pubKey[:]))
}

// Bytes marshals the PubKey using amino encoding.
func (pubKey PubKeySr25519) Bytes() []byte {
	bz, err := cdc.MarshalBinaryBare(pubKey)
	if err != nil {
		panic(err)
	}
	return bz
}

func (pubKey PubKeySr25519) VerifyBytes(msg []byte, sig []byte) bool {
	// make sure we use the same algorithm to sign
	if len(sig) != SignatureSize {
		return false
	}
	var sig64 [SignatureSize]byte
	copy(sig64[:], sig)

	publicKey := &(schnorrkel.PublicKey{})
	err := publicKey.Decode(pubKey)
	if err != nil {
		return false
	}

	signingContext := schnorrkel.NewSigningContext([]byte{}, msg)

	signature := &(schnorrkel.Signature{})
	err = signature.Decode(sig64)
	if err != nil {
		return false
	}

	return publicKey.Verify(signature, signingContext)
}

func (pubKey PubKeySr25519) String() string {
	return fmt.Sprintf("PubKeySr25519{%X}", pubKey[:])
}

// nolint: golint
func (pubKey PubKeySr25519) Equals(other crypto.PubKey) bool {
	if otherEd, ok := other.(PubKeySr25519); ok {
		return bytes.Equal(pubKey[:], otherEd[:])
	} else {
		return false
	}
}
