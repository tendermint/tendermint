package proto

import (
	"fmt"

	proto "github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/crypto/sr25519"
)

// TODO: add prefixs for keys

func MarshalPubKey(pki crypto.PubKey) ([]byte, error) {
	protoKey := GetProtoPubKey(pki)
	return proto.Marshal(&protoKey)
}

func GetProtoPubKey(pki crypto.PubKey) PubKey {
	var asOneof isPubKey_Key
	switch pki := pki.(type) {
	case ed25519.PubKey:
		asOneof = &PubKey_Ed25519{Ed25519: pki[:]}
	case sr25519.PubKey:
		asOneof = &PubKey_Sr25519{Sr25519: pki[:]}
	case secp256k1.PubKey:
		asOneof = &PubKey_Secp256K1{Secp256K1: pki[:]}
		// TODO: proto currently is not implmented for multisig
	// case multisig.PubKeyMultisigThreshold:

	// asOneof = &crypto.PubKey_Multisig{
	// 	&crypto.PubKeyMultiSigThreshold{
	// 		K:       uint64(pki.K),
	// 		PubKeys: pki.PubKeys,
	// 	},
	// }
	default:
		panic("can not get protokey of unknown type")
	}

	protoKey := PubKey{
		Key: asOneof,
	}
	return protoKey
}

func UnmarshalPubKey(bz []byte, dest *crypto.PubKey) error {
	var protoKey PubKey
	err := proto.Unmarshal(bz, &protoKey)
	if err != nil {
		return err
	}
	key, err := GetPubKey(protoKey)
	if err != nil {
		return err
	}
	*dest = key
	return nil
}

func GetPubKey(protoKey PubKey) (crypto.PubKey, error) {
	switch asOneof := protoKey.Key.(type) {
	case *PubKey_Ed25519:
		if len(protoKey.GetEd25519()) != ed25519.PubKeySize {
			return nil, fmt.Errorf("invalid size for PubKeyEd25519. Got %d, expected %d",
				len(protoKey.GetEd25519()), ed25519.PubKeySize)
		}
		var key ed25519.PubKey
		copy(key[:], protoKey.GetEd25519())
		return key, nil
	case *PubKey_Secp256K1:
		if len(protoKey.GetSecp256K1()) != secp256k1.PubKeySize {
			return nil, fmt.Errorf("invalid size for PubKeySecp256K1. Got %d, expected %d",
				len(protoKey.GetSecp256K1()), secp256k1.PubKeySize)
		}
		var key secp256k1.PubKey
		copy(key[:], protoKey.GetSecp256K1())
		return key, nil
	case *PubKey_Sr25519:
		if len(protoKey.GetSr25519()) != sr25519.PubKeySize {
			return nil, fmt.Errorf("invalid size for PubKeySr25519. Got %d, expected %d",
				len(protoKey.GetSr25519()), sr25519.PubKeySize)
		}
		var key sr25519.PubKey
		copy(key[:], protoKey.GetSr25519())
		return key, nil
	default:
		return nil, fmt.Errorf("key type not supported: %t", asOneof)
	}
}

func MarshalPrivKey(pki crypto.PrivKey) ([]byte, error) {
	fmt.Println(pki)
	var asOneof isPrivKey_Key
	switch pki := pki.(type) {
	case ed25519.PrivKey:
		asOneof = &PrivKey_Ed25519{Ed25519: pki}
	case sr25519.PrivKey:
		asOneof = &PrivKey_Sr25519{Sr25519: pki[:]}
	case secp256k1.PrivKey:
		asOneof = &PrivKey_Secp256K1{Secp256K1: pki}
	}

	protoKey := PrivKey{
		Key: asOneof,
	}
	return proto.Marshal(&protoKey)
}

func UnmarshalPrivKey(bz []byte, dest *crypto.PrivKey) error {
	var protoKey PrivKey
	err := proto.Unmarshal(bz, &protoKey)
	if err != nil {
		return err
	}
	switch asOneof := protoKey.Key.(type) {
	case *PrivKey_Ed25519:
		if len(protoKey.GetEd25519()) != ed25519.PrivateKeySize {
			return fmt.Errorf("invalid size for PrivKey Ed25519. Got %d, expected %d",
				len(protoKey.GetEd25519()), ed25519.PrivateKeySize)
		}
		var key ed25519.PrivKey = protoKey.GetEd25519()
		*dest = key
	case *PrivKey_Secp256K1:
		if len(protoKey.GetSecp256K1()) != secp256k1.PrivKeySize {
			return fmt.Errorf("invalid size for PrivKey Secp256K1. Got %d, expected %d",
				len(protoKey.GetSecp256K1()), secp256k1.PrivKeySize)
		}
		var key secp256k1.PrivKey = protoKey.GetSecp256K1()
		*dest = key
	case *PrivKey_Sr25519:
		if len(protoKey.GetSr25519()) != sr25519.PrivKeySize {
			return fmt.Errorf("invalid size for PrivKey Sr25519. Got %d, expected %d",
				len(protoKey.GetSr25519()), sr25519.PrivKeySize)
		}
		var key sr25519.PrivKey
		copy(key[:], protoKey.GetSr25519())
		*dest = key
	default:
		fmt.Println(asOneof, "asoneof")
	}

	return nil
}
