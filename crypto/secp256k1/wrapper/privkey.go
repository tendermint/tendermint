package wrapper

import (
	"crypto/elliptic"
	secp256k1 "github.com/btcsuite/btcd/btcec"
)

type PrivateKey secp256k1.PrivateKey

func PrivKeyFromBytes(curve elliptic.Curve, pk []byte) (*PrivateKey,
	*secp256k1.PublicKey) {
	priv, pub := secp256k1.PrivKeyFromBytes(curve, pk)
	return (*PrivateKey)(priv), pub
}

func (p *PrivateKey) Sign(hash []byte) (*Signature, error) {
	origSig, err := (*secp256k1.PrivateKey)(p).Sign(hash)
	return &Signature{*origSig}, err
}
