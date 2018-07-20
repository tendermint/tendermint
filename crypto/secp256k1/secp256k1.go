package secp256k1

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"

	secp256k1 "github.com/btcsuite/btcd/btcec"
	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/common"
	"golang.org/x/crypto/ripemd160"
)

//-------------------------------------
const (
	Secp256k1PrivKeyAminoRoute   = "tendermint/PrivKeySecp256k1"
	Secp256k1PubKeyAminoRoute    = "tendermint/PubKeySecp256k1"
	Secp256k1SignatureAminoRoute = "tendermint/SignatureSecp256k1"
)

var cdc = amino.NewCodec()

func init() {
	// NOTE: It's important that there be no conflicts here,
	// as that would change the canonical representations,
	// and therefore change the address.
	// TODO: Add feature to go-amino to ensure that there
	// are no conflicts.
	cdc.RegisterInterface((*crypto.PubKey)(nil), nil)
	cdc.RegisterConcrete(PubKeySecp256k1{},
		Secp256k1PubKeyAminoRoute, nil)

	cdc.RegisterInterface((*crypto.PrivKey)(nil), nil)
	cdc.RegisterConcrete(PrivKeySecp256k1{},
		Secp256k1PrivKeyAminoRoute, nil)

	cdc.RegisterInterface((*crypto.Signature)(nil), nil)
	cdc.RegisterConcrete(SignatureSecp256k1{},
		Secp256k1SignatureAminoRoute, nil)
}

//-------------------------------------

var _ crypto.PrivKey = PrivKeySecp256k1{}

// Implements PrivKey
type PrivKeySecp256k1 [32]byte

func (privKey PrivKeySecp256k1) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(privKey)
}

func (privKey PrivKeySecp256k1) Sign(msg []byte) (crypto.Signature, error) {
	priv__, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey[:])
	sig__, err := priv__.Sign(crypto.Sha256(msg))
	if err != nil {
		return nil, err
	}
	return SignatureSecp256k1(sig__.Serialize()), nil
}

func (privKey PrivKeySecp256k1) PubKey() crypto.PubKey {
	_, pub__ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey[:])
	var pub PubKeySecp256k1
	copy(pub[:], pub__.SerializeCompressed())
	return pub
}

// Equals - you probably don't need to use this.
// Runs in constant time based on length of the keys.
func (privKey PrivKeySecp256k1) Equals(other crypto.PrivKey) bool {
	if otherSecp, ok := other.(PrivKeySecp256k1); ok {
		return subtle.ConstantTimeCompare(privKey[:], otherSecp[:]) == 1
	} else {
		return false
	}
}

/*
// Deterministically generates new priv-key bytes from key.
func (key PrivKeySecp256k1) Generate(index int) PrivKeySecp256k1 {
	newBytes := cdc.BinarySha256(struct {
		PrivKey [64]byte
		Index   int
	}{key, index})
	var newKey [64]byte
	copy(newKey[:], newBytes)
	return PrivKeySecp256k1(newKey)
}
*/

func GenPrivKey() PrivKeySecp256k1 {
	privKeyBytes := [32]byte{}
	copy(privKeyBytes[:], crypto.CRandBytes(32))
	priv, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKeyBytes[:])
	copy(privKeyBytes[:], priv.Serialize())
	return PrivKeySecp256k1(privKeyBytes)
}

// NOTE: secret should be the output of a KDF like bcrypt,
// if it's derived from user input.
func GenPrivKeyFromSecret(secret []byte) PrivKeySecp256k1 {
	privKey32 := crypto.Sha256(secret) // Not Ripemd160 because we want 32 bytes.
	priv, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey32)
	privKeyBytes := [32]byte{}
	copy(privKeyBytes[:], priv.Serialize())
	return PrivKeySecp256k1(privKeyBytes)
}

//-------------------------------------

var _ crypto.PubKey = PubKeySecp256k1{}

const PubKeySecp256k1Size = 33

// Implements crypto.PubKey.
// Compressed pubkey (just the x-cord),
// prefixed with 0x02 or 0x03, depending on the y-cord.
type PubKeySecp256k1 [PubKeySecp256k1Size]byte

// Implements Bitcoin style addresses: RIPEMD160(SHA256(pubkey))
func (pubKey PubKeySecp256k1) Address() crypto.Address {
	hasherSHA256 := sha256.New()
	hasherSHA256.Write(pubKey[:]) // does not error
	sha := hasherSHA256.Sum(nil)

	hasherRIPEMD160 := ripemd160.New()
	hasherRIPEMD160.Write(sha) // does not error
	return crypto.Address(hasherRIPEMD160.Sum(nil))
}

func (pubKey PubKeySecp256k1) Bytes() []byte {
	bz, err := cdc.MarshalBinaryBare(pubKey)
	if err != nil {
		panic(err)
	}
	return bz
}

func (pubKey PubKeySecp256k1) VerifyBytes(msg []byte, sig_ crypto.Signature) bool {
	// and assert same algorithm to sign and verify
	sig, ok := sig_.(SignatureSecp256k1)
	if !ok {
		return false
	}

	pub__, err := secp256k1.ParsePubKey(pubKey[:], secp256k1.S256())
	if err != nil {
		return false
	}
	sig__, err := secp256k1.ParseDERSignature(sig[:], secp256k1.S256())
	if err != nil {
		return false
	}
	return sig__.Verify(crypto.Sha256(msg), pub__)
}

func (pubKey PubKeySecp256k1) String() string {
	return fmt.Sprintf("PubKeySecp256k1{%X}", pubKey[:])
}

func (pubKey PubKeySecp256k1) Equals(other crypto.PubKey) bool {
	if otherSecp, ok := other.(PubKeySecp256k1); ok {
		return bytes.Equal(pubKey[:], otherSecp[:])
	} else {
		return false
	}
}

//-------------------------------------

var _ crypto.Signature = SignatureSecp256k1{}

// Implements crypto.Signature
type SignatureSecp256k1 []byte

func (sig SignatureSecp256k1) Bytes() []byte {
	bz, err := cdc.MarshalBinaryBare(sig)
	if err != nil {
		panic(err)
	}
	return bz
}

func (sig SignatureSecp256k1) IsZero() bool { return len(sig) == 0 }

func (sig SignatureSecp256k1) String() string {
	return fmt.Sprintf("/%X.../", common.Fingerprint(sig[:]))
}

func (sig SignatureSecp256k1) Equals(other crypto.Signature) bool {
	if otherSecp, ok := other.(SignatureSecp256k1); ok {
		return subtle.ConstantTimeCompare(sig[:], otherSecp[:]) == 1
	} else {
		return false
	}
}

func SignatureSecp256k1FromBytes(data []byte) crypto.Signature {
	sig := make(SignatureSecp256k1, len(data))
	copy(sig[:], data)
	return sig
}
