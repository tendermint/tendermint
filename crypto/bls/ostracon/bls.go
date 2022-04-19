package bls

import (
	"bytes"
	"crypto/sha512"
	"crypto/subtle"
	"fmt"

	tmjson "github.com/line/ostracon/libs/json"

	"github.com/herumi/bls-eth-go-binary/bls"

	"github.com/line/ostracon/crypto"
	"github.com/line/ostracon/crypto/tmhash"
)

var _ crypto.PrivKey = PrivKey{}

const (
	PrivKeyName   = "ostracon/PrivKeyBLS12"
	PubKeyName    = "ostracon/PubKeyBLS12"
	PrivKeySize   = 32
	PubKeySize    = 48
	SignatureSize = 96
	KeyType       = "bls12-381"
)

func init() {
	tmjson.RegisterType(PubKey{}, PubKeyName)
	tmjson.RegisterType(PrivKey{}, PrivKeyName)

	err := bls.Init(bls.BLS12_381)
	if err != nil {
		panic(fmt.Sprintf("ERROR: %s", err))
	}
	err = bls.SetETHmode(bls.EthModeLatest)
	if err != nil {
		panic(fmt.Sprintf("ERROR: %s", err))
	}
}

// PrivKey implements crypto.PrivKey.
type PrivKey [PrivKeySize]byte

// AddSignature adds a BLS signature to the init. When the init is nil, then a new aggregate signature is built
// from specified signature.
func AddSignature(init []byte, signature []byte) (aggrSign []byte, err error) {
	if init == nil {
		blsSign := bls.Sign{}
		init = blsSign.Serialize()
	} else if len(init) != SignatureSize {
		err = fmt.Errorf("invalid BLS signature: aggregated signature size %d is not valid size %d",
			len(init), SignatureSize)
		return
	}
	if len(signature) != SignatureSize {
		err = fmt.Errorf("invalid BLS signature: signature size %d is not valid size %d",
			len(signature), SignatureSize)
		return
	}
	blsSign := bls.Sign{}
	err = blsSign.Deserialize(signature)
	if err != nil {
		return
	}
	aggrBLSSign := bls.Sign{}
	err = aggrBLSSign.Deserialize(init)
	if err != nil {
		return
	}
	aggrBLSSign.Add(&blsSign)
	aggrSign = aggrBLSSign.Serialize()
	return
}

func VerifyAggregatedSignature(aggregatedSignature []byte, pubKeys []PubKey, msgs [][]byte) error {
	if len(pubKeys) != len(msgs) {
		return fmt.Errorf("the number of public keys %d doesn't match the one of messages %d",
			len(pubKeys), len(msgs))
	}
	if aggregatedSignature == nil {
		if len(pubKeys) == 0 {
			return nil
		}
		return fmt.Errorf(
			"the aggregate signature was omitted, even though %d public keys were specified", len(pubKeys))
	}
	aggrSign := bls.Sign{}
	err := aggrSign.Deserialize(aggregatedSignature)
	if err != nil {
		return err
	}
	blsPubKeys := make([]bls.PublicKey, len(pubKeys))
	hashes := make([][]byte, len(msgs))
	for i := 0; i < len(pubKeys); i++ {
		blsPubKeys[i] = bls.PublicKey{}
		err = blsPubKeys[i].Deserialize(pubKeys[i][:])
		if err != nil {
			return err
		}
		hash := sha512.Sum512_256(msgs[i])
		hashes[i] = hash[:]
	}
	if !aggrSign.VerifyAggregateHashes(blsPubKeys, hashes) {
		return fmt.Errorf("failed to verify the aggregated hashes by %d public keys", len(blsPubKeys))
	}
	return nil
}

// GenPrivKey generates a new BLS12-381 private key.
func GenPrivKey() PrivKey {
	sigKey := bls.SecretKey{}
	sigKey.SetByCSPRNG()
	sigKeyBinary := PrivKey{}
	binary := sigKey.Serialize()
	if len(binary) != PrivKeySize {
		panic(fmt.Sprintf("unexpected BLS private key size: %d != %d", len(binary), PrivKeySize))
	}
	copy(sigKeyBinary[:], binary)
	return sigKeyBinary
}

// Bytes marshals the privkey using amino encoding.
func (privKey PrivKey) Bytes() []byte {
	return privKey[:]
}

// Sign produces a signature on the provided message.
func (privKey PrivKey) Sign(msg []byte) ([]byte, error) {
	if msg == nil {
		panic("Nil specified as the message")
	}
	blsKey := bls.SecretKey{}
	err := blsKey.Deserialize(privKey[:])
	if err != nil {
		return nil, err
	}
	hash := sha512.Sum512_256(msg)
	sign := blsKey.SignHash(hash[:])
	return sign.Serialize(), nil
}

// VRFProve is not supported in BLS12.
func (privKey PrivKey) VRFProve(seed []byte) (crypto.Proof, error) {
	return nil, fmt.Errorf("VRF prove is not supported by the BLS12")
}

// PubKey gets the corresponding public key from the private key.
func (privKey PrivKey) PubKey() crypto.PubKey {
	blsKey := bls.SecretKey{}
	err := blsKey.Deserialize(privKey[:])
	if err != nil {
		panic(fmt.Sprintf("Not a BLS12-381 private key: %X", privKey[:]))
	}
	pubKey := blsKey.GetPublicKey()
	pubKeyBinary := PubKey{}
	binary := pubKey.Serialize()
	if len(binary) != PubKeySize {
		panic(fmt.Sprintf("unexpected BLS public key size: %d != %d", len(binary), PubKeySize))
	}
	copy(pubKeyBinary[:], binary)
	return pubKeyBinary
}

// Equals - you probably don't need to use this.
// Runs in constant time based on length of the keys.
func (privKey PrivKey) Equals(other crypto.PrivKey) bool {
	if otherEd, ok := other.(PrivKey); ok {
		return subtle.ConstantTimeCompare(privKey[:], otherEd[:]) == 1
	}
	return false
}

// Type returns information to identify the type of this key.
func (privKey PrivKey) Type() string {
	return KeyType
}

var _ crypto.PubKey = PubKey{}

// PubKey implements crypto.PubKey for the BLS12-381 signature scheme.
type PubKey [PubKeySize]byte

// Address is the SHA256-20 of the raw pubkey bytes.
func (pubKey PubKey) Address() crypto.Address {
	return tmhash.SumTruncated(pubKey[:])
}

// Bytes marshals the PubKey using amino encoding.
func (pubKey PubKey) Bytes() []byte {
	return pubKey[:]
}

func (pubKey PubKey) VerifySignature(msg []byte, sig []byte) bool {
	// make sure we use the same algorithm to sign
	if len(sig) != SignatureSize {
		return false
	}
	blsPubKey := bls.PublicKey{}
	err := blsPubKey.Deserialize(pubKey[:])
	if err != nil {
		return false
	}
	blsSign := bls.Sign{}
	err = blsSign.Deserialize(sig)
	if err != nil {
		fmt.Printf("Signature Deserialize failed: %s", err)
		return false
	}
	hash := sha512.Sum512_256(msg)
	return blsSign.VerifyHash(&blsPubKey, hash[:])
}

// VRFVerify is not supported in BLS12.
func (pubKey PubKey) VRFVerify(proof crypto.Proof, seed []byte) (crypto.Output, error) {
	return nil, fmt.Errorf("VRF verify is not supported by the BLS12")
}

func (pubKey PubKey) String() string {
	return fmt.Sprintf("PubKeyBLS12{%X}", pubKey[:])
}

func (pubKey PubKey) Equals(other crypto.PubKey) bool {
	if otherEd, ok := other.(PubKey); ok {
		return bytes.Equal(pubKey[:], otherEd[:])
	}
	return false
}

// Type returns information to identify the type of this key.
func (pubKey PubKey) Type() string {
	return KeyType
}
