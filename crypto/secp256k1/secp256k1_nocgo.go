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
	priv, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey[:])
	sig, err := priv.Sign(crypto.Sha256(msg))
	if err != nil {
		return nil, err
	}
	// TODO: this serializes to
	//  0x30 <length> 0x02 <length r> r 0x02 <length s> s
	// instead of r || s as we want
	// use SignCompact instead and re-benchmark
	return sig.Serialize(), nil
}

func (pubKey PubKeySecp256k1) VerifyBytes(msg []byte, sig []byte) bool {
	pub, err := secp256k1.ParsePubKey(pubKey[:], secp256k1.S256())
	if err != nil {
		return false
	}
	// this will only parse correctly if the signature is in the form
	//  0x30 <length> 0x02 <length r> r 0x02 <length s> s
	// we want to accept sigs which are r || s (with padded r, s) and in lower-S form
	parsedSig, err := secp256k1.ParseSignature(sig[:], secp256k1.S256())
	if err != nil {
		return false
	}
	// Reject malleable signatures. libsecp256k1 does this check but btcec doesn't.
	// see: https://github.com/ethereum/go-ethereum/blob/f9401ae011ddf7f8d2d95020b7446c17f8d98dc1/crypto/signature_nocgo.go#L90-L93
	if parsedSig.S.Cmp(secp256k1halfN) > 0 {
		return false
	}
	return parsedSig.Verify(crypto.Sha256(msg), pub)
}
