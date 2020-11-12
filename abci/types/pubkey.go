package types

import (
	"github.com/tendermint/tendermint/crypto/ed25519"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	tmcrypto "github.com/tendermint/tendermint/proto/tendermint/crypto"
)

func UpdateValidator(pk []byte, power int64, keyType string) ValidatorUpdate {
	var (
		pkp tmcrypto.PublicKey
		err error
	)

	switch keyType {
	case "", ed25519.KeyType:
		pke := ed25519.PubKey(pk)
		pkp, err = cryptoenc.PubKeyToProto(pke)
		if err != nil {
			panic(err)
		}
	case secp256k1.KeyType:
		pke := secp256k1.PubKey(pk)
		pkp, err = cryptoenc.PubKeyToProto(pke)
		if err != nil {
			panic(err)
		}
	}

	return ValidatorUpdate{
		// Address:
		PubKey: pkp,
		Power:  power,
	}
}
