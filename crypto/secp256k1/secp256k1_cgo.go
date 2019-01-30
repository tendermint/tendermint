// +build cgo

package secp256k1

import (
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/tendermint/tendermint/crypto"
)

// Sign creates an ECDSA signature on curve Secp256k1, using SHA256 on the msg.
func (privKey PrivKeySecp256k1) Sign(msg []byte) ([]byte, error) {
	// TODO: if we don't want the additional computation here
	//  C.secp256k1_ecdsa_recoverable_signature_serialize_compact(context, sigdata, &recid, &sigstruct))
	//  we'd need to use the C library directly
	//
	// i.e. secp256k1.Sign calls
	//  - C.secp256k1_ecdsa_sign_recoverable(context, sigdata, &recid, &sigstruct))
	//  - C.secp256k1_ecdsa_recoverable_signature_serialize_compact(context, sigdata, &recid, &sigstruct)
	//
	// -> benchmark against what we had before (golang only) and the direct calls to the C API
	rsv, err := secp256k1.Sign(crypto.Sha256(msg), privKey[:])
	if err != nil {
		return nil, err
	}
	// we do not need v  in r||s||v:
	rs := rsv[:len(rsv)-1]
	return rs, nil
}

func (pubKey PubKeySecp256k1) VerifyBytes(msg []byte, sig []byte) bool {
	return secp256k1.VerifySignature(pubKey[:], crypto.Sha256(msg), sig)
}
