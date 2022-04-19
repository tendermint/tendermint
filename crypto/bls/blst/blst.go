package blst

import (
	"crypto/rand"

	bls "github.com/supranational/blst/bindings/go"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/bytes"
)

const (
	PrivKeyName   = "blst/PrivKey"
	PubKeyName    = "blst/PubKey"
	PrivKeySize   = 32
	PubKeySize    = 192
	SignatureSize = 96
)

var dst = []byte("BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_NUL_")

// PrivKey implements tendermint.crypto.PrivKey.
type PrivKey struct {
	sk *bls.SecretKey
}

func GenPrivKey() *PrivKey {
	var ikm [32]byte
	_, _ = rand.Read(ikm[:])
	return &PrivKey{bls.KeyGen(ikm[:])}
}

func (k *PrivKey) Bytes() []byte {
	return k.sk.Serialize()
}

func (k *PrivKey) Equals(other crypto.PrivKey) bool {
	if otherK, ok := other.(*PrivKey); ok {
		return k.sk.Equals(otherK.sk)
	}
	return false
}

func (k *PrivKey) PubKey() crypto.PubKey {
	return &PubKey{new(bls.P2Affine).From(k.sk)}
}

func (k *PrivKey) Sign(msg []byte) ([]byte, error) {
	sig := new(bls.P1Affine).Sign(k.sk, msg, dst)
	return sig.Serialize(), nil
}

func (k *PrivKey) Type() string {
	return PrivKeyName
}

// PubKey implements tendermint.crypto.PubKey.
type PubKey struct {
	pk *bls.P2Affine
}

func (k *PubKey) Address() bytes.HexBytes {
	return tmhash.SumTruncated(k.Bytes())
}

func (k *PubKey) Bytes() []byte {
	return k.pk.Serialize()
}

func (k *PubKey) Equals(other crypto.PubKey) bool {
	if otherK, ok := other.(*PubKey); ok {
		return k.pk.Equals(otherK.pk)
	}
	return false
}

func (k *PubKey) Type() string {
	return PubKeyName
}

func (k *PubKey) VerifySignature(msg []byte, bsig []byte) bool {
	sig := new(bls.P1Affine).Deserialize(bsig)
	if sig == nil {
		return false
	}
	return sig.Verify(true, k.pk, false, msg, dst)
}
