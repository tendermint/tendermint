package secp256k1

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"io"
	"math/big"

	secp256k1 "github.com/btcsuite/btcd/btcec"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/jsontypes"

	// necessary for Bitcoin address format
	"golang.org/x/crypto/ripemd160" //nolint:staticcheck
)

//-------------------------------------
const (
	PrivKeyName = "tendermint/PrivKeySecp256k1"
	PubKeyName  = "tendermint/PubKeySecp256k1"

	KeyType     = "secp256k1"
	PrivKeySize = 32
)

func init() {
	jsontypes.MustRegister(PubKey{})
	jsontypes.MustRegister(PrivKey{})
}

var _ crypto.PrivKey = PrivKey{}

// PrivKey implements PrivKey.
type PrivKey []byte

// TypeTag satisfies the jsontypes.Tagged interface.
func (PrivKey) TypeTag() string { return PrivKeyName }

// Bytes marshalls the private key using amino encoding.
func (privKey PrivKey) Bytes() []byte {
	return []byte(privKey)
}

// PubKey performs the point-scalar multiplication from the privKey on the
// generator point to get the pubkey.
func (privKey PrivKey) PubKey() crypto.PubKey {
	_, pubkeyObject := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey)

	pk := pubkeyObject.SerializeCompressed()

	return PubKey(pk)
}

// Equals - you probably don't need to use this.
// Runs in constant time based on length of the keys.
func (privKey PrivKey) Equals(other crypto.PrivKey) bool {
	if otherSecp, ok := other.(PrivKey); ok {
		return subtle.ConstantTimeCompare(privKey[:], otherSecp[:]) == 1
	}
	return false
}

func (privKey PrivKey) Type() string {
	return KeyType
}

// GenPrivKey generates a new ECDSA private key on curve secp256k1 private key.
// It uses OS randomness to generate the private key.
func GenPrivKey() PrivKey {
	return genPrivKey(rand.Reader)
}

// genPrivKey generates a new secp256k1 private key using the provided reader.
func genPrivKey(rand io.Reader) PrivKey {
	var privKeyBytes [PrivKeySize]byte
	d := new(big.Int)

	for {
		privKeyBytes = [PrivKeySize]byte{}
		_, err := io.ReadFull(rand, privKeyBytes[:])
		if err != nil {
			panic(err)
		}

		d.SetBytes(privKeyBytes[:])
		// break if we found a valid point (i.e. > 0 and < N == curverOrder)
		isValidFieldElement := 0 < d.Sign() && d.Cmp(secp256k1.S256().N) < 0
		if isValidFieldElement {
			break
		}
	}

	return PrivKey(privKeyBytes[:])
}

var one = new(big.Int).SetInt64(1)

// GenPrivKeySecp256k1 hashes the secret with SHA2, and uses
// that 32 byte output to create the private key.
//
// It makes sure the private key is a valid field element by setting:
//
// c = sha256(secret)
// k = (c mod (n − 1)) + 1, where n = curve order.
//
// NOTE: secret should be the output of a KDF like bcrypt,
// if it's derived from user input.
func GenPrivKeySecp256k1(secret []byte) PrivKey {
	secHash := sha256.Sum256(secret)
	// to guarantee that we have a valid field element, we use the approach of:
	// "Suite B Implementer’s Guide to FIPS 186-3", A.2.1
	// https://apps.nsa.gov/iaarchive/library/ia-guidance/ia-solutions-for-classified/algorithm-guidance/suite-b-implementers-guide-to-fips-186-3-ecdsa.cfm
	// see also https://github.com/golang/go/blob/0380c9ad38843d523d9c9804fe300cb7edd7cd3c/src/crypto/ecdsa/ecdsa.go#L89-L101
	fe := new(big.Int).SetBytes(secHash[:])
	n := new(big.Int).Sub(secp256k1.S256().N, one)
	fe.Mod(fe, n)
	fe.Add(fe, one)

	feB := fe.Bytes()
	privKey32 := make([]byte, PrivKeySize)
	// copy feB over to fixed 32 byte privKey32 and pad (if necessary)
	copy(privKey32[32-len(feB):32], feB)

	return PrivKey(privKey32)
}

//-------------------------------------

var _ crypto.PubKey = PubKey{}

// PubKeySize is comprised of 32 bytes for one field element
// (the x-coordinate), plus one byte for the parity of the y-coordinate.
const PubKeySize = 33

// PubKey implements crypto.PubKey.
// It is the compressed form of the pubkey. The first byte depends is a 0x02 byte
// if the y-coordinate is the lexicographically largest of the two associated with
// the x-coordinate. Otherwise the first byte is a 0x03.
// This prefix is followed with the x-coordinate.
type PubKey []byte

// TypeTag satisfies the jsontypes.Tagged interface.
func (PubKey) TypeTag() string { return PubKeyName }

// Address returns a Bitcoin style addresses: RIPEMD160(SHA256(pubkey))
func (pubKey PubKey) Address() crypto.Address {
	if len(pubKey) != PubKeySize {
		panic("length of pubkey is incorrect")
	}
	hasherSHA256 := sha256.New()
	_, _ = hasherSHA256.Write(pubKey) // does not error
	sha := hasherSHA256.Sum(nil)

	hasherRIPEMD160 := ripemd160.New()
	_, _ = hasherRIPEMD160.Write(sha) // does not error

	return crypto.Address(hasherRIPEMD160.Sum(nil))
}

// Bytes returns the pubkey marshaled with amino encoding.
func (pubKey PubKey) Bytes() []byte {
	return []byte(pubKey)
}

func (pubKey PubKey) String() string {
	return fmt.Sprintf("PubKeySecp256k1{%X}", []byte(pubKey))
}

func (pubKey PubKey) Equals(other crypto.PubKey) bool {
	if otherSecp, ok := other.(PubKey); ok {
		return bytes.Equal(pubKey[:], otherSecp[:])
	}
	return false
}

func (pubKey PubKey) Type() string {
	return KeyType
}

// used to reject malleable signatures
// see:
//  - https://github.com/ethereum/go-ethereum/blob/f9401ae011ddf7f8d2d95020b7446c17f8d98dc1/crypto/signature_nocgo.go#L90-L93
//  - https://github.com/ethereum/go-ethereum/blob/f9401ae011ddf7f8d2d95020b7446c17f8d98dc1/crypto/crypto.go#L39
var secp256k1halfN = new(big.Int).Rsh(secp256k1.S256().N, 1)

// Sign creates an ECDSA signature on curve Secp256k1, using SHA256 on the msg.
// The returned signature will be of the form R || S (in lower-S form).
func (privKey PrivKey) Sign(msg []byte) ([]byte, error) {
	priv, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey)
	seed := sha256.Sum256(msg)
	sig, err := priv.Sign(seed[:])
	if err != nil {
		return nil, err
	}

	sigBytes := serializeSig(sig)
	return sigBytes, nil
}

// VerifySignature verifies a signature of the form R || S.
// It rejects signatures which are not in lower-S form.
func (pubKey PubKey) VerifySignature(msg []byte, sigStr []byte) bool {
	if len(sigStr) != 64 {
		return false
	}

	pub, err := secp256k1.ParsePubKey(pubKey, secp256k1.S256())
	if err != nil {
		return false
	}

	// parse the signature:
	signature := signatureFromBytes(sigStr)
	// Reject malleable signatures. libsecp256k1 does this check but btcec doesn't.
	// see: https://github.com/ethereum/go-ethereum/blob/f9401ae011ddf7f8d2d95020b7446c17f8d98dc1/crypto/signature_nocgo.go#L90-L93
	if signature.S.Cmp(secp256k1halfN) > 0 {
		return false
	}

	seed := sha256.Sum256(msg)
	return signature.Verify(seed[:], pub)
}

// Read Signature struct from R || S. Caller needs to ensure
// that len(sigStr) == 64.
func signatureFromBytes(sigStr []byte) *secp256k1.Signature {
	return &secp256k1.Signature{
		R: new(big.Int).SetBytes(sigStr[:32]),
		S: new(big.Int).SetBytes(sigStr[32:64]),
	}
}

// Serialize signature to R || S.
// R, S are padded to 32 bytes respectively.
func serializeSig(sig *secp256k1.Signature) []byte {
	rBytes := sig.R.Bytes()
	sBytes := sig.S.Bytes()
	sigBytes := make([]byte, 64)
	// 0 pad the byte arrays from the left if they aren't big enough.
	copy(sigBytes[32-len(rBytes):32], rBytes)
	copy(sigBytes[64-len(sBytes):64], sBytes)
	return sigBytes
}
