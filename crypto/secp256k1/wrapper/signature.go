package wrapper

import (
	"bytes"
	"crypto/elliptic"
	"errors"
	"math/big"

	secp256k1 "github.com/btcsuite/btcd/btcec"
)

var s256halfOrder = new(big.Int).Rsh(secp256k1.S256().N, 1)

type Signature struct {
	secp256k1.Signature
}

// Serialize returns the ECDSA signature as r || s,
// where both r and s are encoded into 32 byte big endian integers.
// In order to remove malleability,
// we set s = curve_order - s, if s is greater than curve.Order() / 2.
func (sig *Signature) Serialize() []byte {
	// TODO we should probably do without:
	//  > this may mean we have to serialize it their way, deserialize it, then reserialize it our way
	//
	origB := sig.Signature.Serialize()
	origSig, err := secp256k1.ParseDERSignature(origB, secp256k1.S256())
	if err != nil {
		panic("bug in underlying secp256k1 implementation")
	}

	rBytes := origSig.R.Bytes()
	sigS := sig.S
	if sigS.Cmp(s256halfOrder) == 1 {
		sigS = new(big.Int).Sub(S256().N, sigS)
	}
	sBytes := origSig.S.Bytes()
	// TODO: remove this check! Just want to make sure this holds:
	if !bytes.Equal(sBytes, sigS.Bytes()) {
		panic("this would be the only reason  to 'serialize it their way, deserialize it, then reserialize it our way'")
	}
	sigBytes := make([]byte, 64)
	// 0 pad the byte arrays from the left if they aren't big enough.
	copy(sigBytes[32-len(rBytes):32], rBytes)
	copy(sigBytes[64-len(sBytes):64], sBytes)
	return sigBytes
}

func ParseSignature(sigStr []byte, curve elliptic.Curve) (*Signature, error) {
	if len(sigStr) != 64 {
		return nil, errors.New("malformed signature: not 64 bytes")
	}
	signature := &Signature{}
	signature.R = new(big.Int).SetBytes(sigStr[:32])
	signature.S = new(big.Int).SetBytes(sigStr[32:64])
	if signature.S.Cmp(s256halfOrder) > 0 {
		return nil, errors.New("signature S is > (curve.N) / 2")
	}

	return signature, nil
}

func (sig *Signature) Verify(hash []byte, pubKey *secp256k1.PublicKey) bool {
	if sig.S.Cmp(s256halfOrder) > 0 {
		return false
	}
	return sig.Signature.Verify(hash, pubKey)
}

func S256() *secp256k1.KoblitzCurve {
	return secp256k1.S256()
}
