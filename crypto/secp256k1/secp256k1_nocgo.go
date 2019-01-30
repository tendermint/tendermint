// +build !cgo

package secp256k1

import (
	"math/big"

	secp256k1 "github.com/btcsuite/btcd/btcec"

	"github.com/tendermint/tendermint/crypto"
)

// used to reject malleable signatures
// see:
//  - https://github.com/ethereum/go-ethereum/blob/f9401ae011ddf7f8d2d95020b7446c17f8d98dc1/crypto/signature_nocgo.go#L90-L93
//  - https://github.com/ethereum/go-ethereum/blob/f9401ae011ddf7f8d2d95020b7446c17f8d98dc1/crypto/crypto.go#L39
var secp256k1N, _ = new(big.Int).SetString("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", 16)
var secp256k1halfN = new(big.Int).Div(secp256k1N, big.NewInt(2))

// Sign creates an ECDSA signature on curve Secp256k1, using SHA256 on the msg.
func (privKey PrivKeySecp256k1) Sign(msg []byte) ([]byte, error) {
	// <(byte of 27+public key solution)+4 if compressed >< padded bytes for signature R><padded bytes for signature S>
	// signature is in lower-S form:
	priv, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey[:])
	vrs, err := secp256k1.SignCompact(secp256k1.S256(), priv, crypto.Sha256(msg), false)
	if err != nil {
		return nil, err
	}
	return vrs[1:], nil
}

func (pubKey PubKeySecp256k1) VerifyBytes(msg []byte, sig []byte) bool {
	if len(sig) != 64 {
		return false
	}
	signature := &secp256k1.Signature{R: new(big.Int).SetBytes(sig[:32]), S: new(big.Int).SetBytes(sig[32:])}
	key, err := secp256k1.ParsePubKey(pubKey[:], secp256k1.S256())
	if err != nil {
		return false
	}
	// Reject malleable signatures. libsecp256k1 does this check but btcec doesn't.
	if signature.S.Cmp(secp256k1halfN) > 0 {
		return false
	}
	return signature.Verify(crypto.Sha256(msg), key)

}
