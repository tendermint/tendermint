package types

import (
	fmt "fmt"

	"github.com/tendermint/tendermint/crypto/ed25519"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

func Ed25519ValidatorUpdate(pk []byte, power int64) ValidatorUpdate {
	pke := ed25519.PubKey(pk)

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

func UpdateValidator(pk []byte, power int64, keyType string) ValidatorUpdate {
	switch keyType {
	case "", ed25519.KeyType:
		return Ed25519ValidatorUpdate(pk, power)
	case secp256k1.KeyType:
		pke := secp256k1.PubKey(pk)
		pkp, err := cryptoenc.PubKeyToProto(pke)
		if err != nil {
			panic(err)
		}
		return ValidatorUpdate{
			// Address:
			PubKey: pkp,
			Power:  power,
		}
	default:
		panic(fmt.Sprintf("key type %s not supported", keyType))
	}
}
