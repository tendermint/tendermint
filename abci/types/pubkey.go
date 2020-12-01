package types

import (
	"github.com/tendermint/tendermint/crypto/bls12381"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
)

const (
	PubKeyEd25519  = "ed25519"
	PubKeyBLS12381 = "bls12381"
)

func UpdateValidator(pk []byte, power int64) ValidatorUpdate {
	pke := bls12381.PubKey(pk)
	pkp, err := cryptoenc.PubKeyToProto(pke)
	if err != nil {
		panic(err)
	}

	return ValidatorUpdate{
		// Address:
		PubKey: pkp,
		Power:  power,
	}
}
