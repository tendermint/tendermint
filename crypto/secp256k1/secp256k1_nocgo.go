// +build !libsecp256k1

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
var secp256k1halfN = new(big.Int).Rsh(secp256k1.S256().N, 1)

// Sign creates an ECDSA signature on curve Secp256k1, using SHA256 on the msg.
// The returned signature will be of the form R || S (in lower-S form).
func (privKey PrivKeySecp256k1) Sign(msg []byte) ([]byte, error) {
	priv, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey[:])
	sig, err := priv.Sign(crypto.Sha256(msg))
	if err != nil {
		return nil, err
	}
	sigBytes := serializeSig(sig)
	return sigBytes, nil
}

// VerifyBytes verifies a signature of the form R || S.
// It rejects signatures which are not in lower-S form.
func (pubKey PubKeySecp256k1) VerifyBytes(msg []byte, sigStr []byte) bool {
	if len(sigStr) == 65 {
		// the least byte should be ethereum signature format with 'recovery id' v
		sigStr = sigStr[0:64]
	}

	if len(sigStr) != 64 {
		return false
	}

	pub, err := secp256k1.ParsePubKey(pubKey[:], secp256k1.S256())
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
	return signature.Verify(crypto.Sha256(msg), pub)
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


// SignRecoverAble creates an ECDSA signature on curve Secp256k1, a recoverable ECDSA signature .
// it returns the result that is in 65-byte [R || S || V]
// the lengeth of hash should must be 32 bytes
func (privKey PrivKeySecp256k1) SignRecoverAble(msg []byte) ([]byte, error) {
	priv, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey[:])
	sig, err := secp256k1.SignCompact(secp256k1.S256(), priv, crypto.Sha256(msg), true)
	if err != nil {
		return nil, err
	}
	// Convert to Ethereum signature format with 'recovery id' v at the end.
	v := sig[0] - 27
	copy(sig, sig[1:])
	sig[64] = v
	return sig, nil
}

// RecoverPubkeyFromSign recover pubkey from signature
// it requires signature is in the 65-byte [R || S || V]
func (privKey PrivKeySecp256k1) RecoverPubkeyFromSign(msg, sig []byte) (crypto.PubKey, error) {
	// Convert to btcec input format with 'recovery id' v at the beginning.
	btcsig := make([]byte, 65)
	btcsig[0] = sig[64] + 27
	copy(btcsig[1:], sig)

	var pubkeyBytes PubKeySecp256k1
	pubkeyObj, _, err := secp256k1.RecoverCompact(secp256k1.S256(), btcsig, crypto.Sha256(msg))
	if err != nil {
		return pubkeyBytes, err
	}

	copy(pubkeyBytes[:], pubkeyObj.SerializeCompressed())
	return pubkeyBytes, nil


}
